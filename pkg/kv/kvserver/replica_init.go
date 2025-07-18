// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package kvserver

import (
	"context"
	"time"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/abortspan"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/allocator/plan"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/closedts/ctpb"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/closedts/tracker"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/concurrency"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvflowcontrol/rac2"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvflowcontrol/replica_rac2"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvserverbase"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvstorage"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/load"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/logstore"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/rafttrace"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/split"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/stateloader"
	"github.com/cockroachdb/cockroach/pkg/raft"
	"github.com/cockroachdb/cockroach/pkg/raft/raftpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/rpc/rpcbase"
	"github.com/cockroachdb/cockroach/pkg/server/telemetry"
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/cockroach/pkg/util/buildutil"
	"github.com/cockroachdb/cockroach/pkg/util/envutil"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/logtags"
)

const (
	splitQueueThrottleDuration = 5 * time.Second
	mergeQueueThrottleDuration = 5 * time.Second
)

// defRaftConnClass is the default rpc.ConnectionClass used for non-system raft
// traffic. Normally it is RaftClass, but can be flipped to DefaultClass if the
// corresponding env variable is true.
//
// NB: for a subset of system ranges, SystemClass is used instead of this.
var defRaftConnClass = func() rpcbase.ConnectionClass {
	if envutil.EnvOrDefaultBool("COCKROACH_RAFT_USE_DEFAULT_CONNECTION_CLASS", false) {
		return rpcbase.DefaultClass
	}
	return rpcbase.RaftClass
}()

// loadInitializedReplicaForTesting loads and constructs an initialized Replica,
// after checking its invariants.
func loadInitializedReplicaForTesting(
	ctx context.Context, store *Store, desc *roachpb.RangeDescriptor, replicaID roachpb.ReplicaID,
) (*Replica, error) {
	if !desc.IsInitialized() {
		return nil, errors.AssertionFailedf("can not load with uninitialized descriptor: %s", desc)
	}
	state, err := kvstorage.LoadReplicaState(ctx, store.TODOEngine(), store.StoreID(), desc, replicaID)
	if err != nil {
		return nil, err
	}

	// No need to wait for previous lease to expire since this is only used in
	// tests and some tests don't expect the extra delay.
	return newInitializedReplica(store, state, false /* waitForPrevLeaseToExpire */)
}

// newInitializedReplica creates an initialized Replica from its loaded state.
func newInitializedReplica(
	store *Store, loaded kvstorage.LoadedReplicaState, waitForPrevLeaseToExpire bool,
) (*Replica, error) {
	r := newUninitializedReplicaWithoutRaftGroup(store, loaded.FullReplicaID())
	r.raftMu.Lock()
	defer r.raftMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.initRaftMuLockedReplicaMuLocked(loaded, waitForPrevLeaseToExpire); err != nil {
		return nil, err
	}

	return r, nil
}

// newUninitializedReplica constructs an uninitialized Replica with the given
// range/replica ID. The returned replica remains uninitialized until
// Replica.initRaftMuLockedReplicaMuLocked() is called, but has a temporary Raft
// group that can be used e.g. for elections or snapshot application.
//
// TODO(#94912): we actually have another initialization path which should be
// refactored: Replica.initFromSnapshotLockedRaftMuLocked().
func newUninitializedReplica(store *Store, id roachpb.FullReplicaID) (*Replica, error) {
	r := newUninitializedReplicaWithoutRaftGroup(store, id)
	r.raftMu.Lock()
	defer r.raftMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.initRaftGroupRaftMuLockedReplicaMuLocked(); err != nil {
		return nil, err
	}
	return r, nil
}

// newUninitializedReplicaWithoutRaftGroup creates a new uninitialized replica
// without a Raft group. Use either newInitializedReplica() or
// newUninitializedReplica() instead. This only exists for
// newInitializedReplica() to avoid creating the Raft group twice (once when
// creating the uninitialized replica, and once when initializing it).
func newUninitializedReplicaWithoutRaftGroup(store *Store, id roachpb.FullReplicaID) *Replica {
	uninitState := stateloader.UninitializedReplicaState(id.RangeID)
	r := &Replica{
		AmbientContext: store.cfg.AmbientCtx,
		RangeID:        id.RangeID,
		replicaID:      id.ReplicaID,
		creationTime:   timeutil.Now(),
		store:          store,
		abortSpan:      abortspan.New(id.RangeID),
		concMgr: concurrency.NewManager(concurrency.Config{
			NodeDesc:           store.nodeDesc,
			RangeDesc:          uninitState.Desc,
			Settings:           store.ClusterSettings(),
			DB:                 store.DB(),
			Clock:              store.Clock(),
			Stopper:            store.Stopper(),
			IntentResolver:     store.intentResolver,
			TxnWaitMetrics:     store.txnWaitMetrics,
			SlowLatchGauge:     store.metrics.SlowLatchRequests,
			LatchWaitDurations: store.metrics.LatchWaitDurations,
			DisableTxnPushing:  store.TestingKnobs().DontPushOnLockConflictError,
			TxnWaitKnobs:       store.TestingKnobs().TxnWaitKnobs,
		}),
		allocatorToken: &plan.AllocatorToken{},
	}
	r.sideTransportClosedTimestamp.init(store.cfg.ClosedTimestampReceiver, id.RangeID)
	r.cachedClosedTimestampPolicy.Store(new(ctpb.RangeClosedTimestampPolicy))

	r.mu.pendingLeaseRequest = makePendingLeaseRequest(r)
	r.mu.quiescent = true
	r.mu.conf = store.cfg.DefaultSpanConfig

	r.mu.proposals = map[kvserverbase.CmdIDKey]*ProposalData{}
	r.mu.checksums = map[uuid.UUID]*replicaChecksum{}
	r.mu.proposalBuf.Init((*replicaProposer)(r), tracker.NewLockfreeTracker(), r.Clock(), r.ClusterSettings())
	r.mu.proposalBuf.testing.dontCloseTimestamps = store.cfg.TestingKnobs.DontCloseTimestamps
	if filter := store.cfg.TestingKnobs.TestingProposalSubmitFilter; filter != nil {
		r.mu.proposalBuf.testing.submitProposalFilter = func(p *ProposalData) (bool, error) {
			var seedID kvserverbase.CmdIDKey
			if p.seedProposal != nil {
				seedID = p.seedProposal.idKey
			}
			// Expose proposal data for external test packages.
			return store.cfg.TestingKnobs.TestingProposalSubmitFilter(kvserverbase.ProposalFilterArgs{
				Ctx:        p.Context(),
				RangeID:    id.RangeID,
				StoreID:    store.StoreID(),
				ReplicaID:  id.ReplicaID,
				Cmd:        p.command,
				QuotaAlloc: p.quotaAlloc,
				CmdID:      p.idKey,
				SeedID:     seedID,
				Req:        p.Request,
			})
		}
	}
	r.mu.proposalBuf.testing.leaseIndexFilter = store.cfg.TestingKnobs.LeaseIndexFilter

	if leaseHistoryMaxEntries > 0 {
		r.leaseHistory = newLeaseHistory(leaseHistoryMaxEntries)
	}

	if store.cfg.StorePool != nil {
		r.loadStats = load.NewReplicaLoad(store.Clock(), store.cfg.StorePool.GetNodeLocalityString)
		split.Init(
			&r.loadBasedSplitter,
			newReplicaSplitConfig(store.ClusterSettings()),
			store.metrics.LoadSplitterMetrics,
			store.rebalanceObjManager.Objective().ToSplitObjective(),
		)
	}
	r.lastProblemRangeReplicateEnqueueTime.Store(store.Clock().PhysicalTime())

	// NB: state will be loaded when the replica gets initialized.
	r.shMu.state = uninitState

	r.rangeStr.store(id.ReplicaID, uninitState.Desc)
	// Add replica log tag - the value is rangeStr.String().
	r.AmbientContext.AddLogTag("r", &r.rangeStr)
	r.raftCtx = logtags.AddTag(r.AnnotateCtx(context.Background()), "raft", nil /* value */)
	// Add replica pointer value. NB: this was historically useful for debugging
	// replica GC issues, but is a distraction at the moment.
	// r.AmbientContext.AddLogTag("@", fmt.Sprintf("%x", unsafe.Pointer(r)))

	r.raftMu.rangefeedCTLagObserver = newRangeFeedCTLagObserver()
	r.raftMu.stateLoader = stateloader.Make(id.RangeID)

	// Initialize all the components of the log storage. The state of the log
	// storage, such as RaftTruncatedState and the last entry ID, will be loaded
	// when the replica is initialized.
	sideloaded := logstore.NewDiskSideloadStorage(
		store.cfg.Settings,
		id.RangeID,
		// NB: sideloaded log entries are persisted in the state engine so that they
		// can be ingested to the state machine locally, when being applied.
		store.StateEngine().GetAuxiliaryDir(),
		store.limiters.BulkIOWriteRate,
		store.StateEngine(),
	)
	r.logStorage = &replicaLogStorage{
		ctx:                r.raftCtx,
		raftEntriesMonitor: store.cfg.RaftEntriesMonitor,
		cache:              store.raftEntryCache,
		onSync:             (*replicaSyncCallback)(r),
		metrics:            store.metrics,
	}
	r.logStorage.mu.RWMutex = (*syncutil.RWMutex)(&r.mu.ReplicaMutex)
	r.logStorage.raftMu.Mutex = &r.raftMu.Mutex
	r.logStorage.ls = &logstore.LogStore{
		RangeID:     id.RangeID,
		Engine:      store.LogEngine(),
		Sideload:    sideloaded,
		StateLoader: r.raftMu.stateLoader.StateLoader,
		// NOTE: use the same SyncWaiter loop for all raft log writes performed by a
		// given range ID, to ensure that callbacks are processed in order.
		SyncWaiter: store.syncWaiters[int(id.RangeID)%len(store.syncWaiters)],
		Settings:   store.cfg.Settings,
		DisableSyncLogWriteToss: buildutil.CrdbTestBuild &&
			store.TestingKnobs().DisableSyncLogWriteToss,
	}

	r.splitQueueThrottle = util.Every(splitQueueThrottleDuration)
	r.mergeQueueThrottle = util.Every(mergeQueueThrottleDuration)

	onTrip := func() {
		telemetry.Inc(telemetryTripAsync)
		r.store.Metrics().ReplicaCircuitBreakerCumTripped.Inc(1)
		store.Metrics().ReplicaCircuitBreakerCurTripped.Inc(1)
	}
	onReset := func() {
		store.Metrics().ReplicaCircuitBreakerCurTripped.Dec(1)
	}
	r.breaker = newReplicaCircuitBreaker(
		store.cfg.Settings, store.stopper, r.AmbientContext, r, onTrip, onReset,
	)
	r.LeaderlessWatcher = newLeaderlessWatcher()
	r.shMu.currentRACv2Mode = r.replicationAdmissionControlModeToUse(context.TODO())

	r.raftMu.msgAppScratchForFlowControl = map[roachpb.ReplicaID][]raftpb.Message{}
	r.raftMu.replicaStateScratchForFlowControl = map[roachpb.ReplicaID]rac2.ReplicaStateInfo{}
	r.flowControlV2 = replica_rac2.NewProcessor(replica_rac2.ProcessorOptions{
		NodeID:            store.NodeID(),
		StoreID:           r.StoreID(),
		RangeID:           r.RangeID,
		ReplicaID:         r.replicaID,
		ReplicaForTesting: (*replicaForRACv2)(r),
		ReplicaMutexAsserter: rac2.MakeReplicaMutexAsserter(
			&r.raftMu.Mutex, (*syncutil.RWMutex)(&r.mu.ReplicaMutex)),
		RaftScheduler:          r.store.scheduler,
		AdmittedPiggybacker:    r.store.cfg.KVFlowAdmittedPiggybacker,
		ACWorkQueue:            r.store.cfg.KVAdmissionController,
		MsgAppSender:           r,
		EvalWaitMetrics:        r.store.cfg.KVFlowEvalWaitMetrics,
		RangeControllerFactory: r.store.kvflowRangeControllerFactory,
		Knobs:                  r.store.TestingKnobs().FlowControlTestingKnobs,
	})
	r.RefreshPolicy(nil)
	return r
}

// setStartKeyLocked sets r.startKey. Note that this field has special semantics
// described on its comment. Callers to this method are initializing an
// uninitialized Replica and hold Replica.mu.
func (r *Replica) setStartKeyLocked(startKey roachpb.RKey) {
	r.mu.AssertHeld()
	if r.startKey != nil {
		log.Fatalf(
			r.AnnotateCtx(context.Background()),
			"start key written twice: was %s, now %s", r.startKey, startKey,
		)
	}
	r.startKey = startKey
}

// initRaftMuLockedReplicaMuLocked initializes the Replica using the state
// loaded from storage. Must not be called more than once on a Replica.
func (r *Replica) initRaftMuLockedReplicaMuLocked(
	s kvstorage.LoadedReplicaState, waitForPrevLeaseToExpire bool,
) error {
	desc := s.ReplState.Desc
	// Ensure that the loaded state corresponds to the same replica.
	if desc.RangeID != r.RangeID || s.ReplicaID != r.replicaID {
		return errors.AssertionFailedf(
			"%s: trying to init with other replica's state r%d/%d", r, desc.RangeID, s.ReplicaID)
	}
	// Ensure that we transition to initialized replica, and do it only once.
	if !desc.IsInitialized() {
		return errors.AssertionFailedf("%s: cannot init replica with uninitialized descriptor", r)
	} else if r.IsInitialized() {
		return errors.AssertionFailedf("%s: cannot reinitialize an initialized replica", r)
	}

	r.setStartKeyLocked(desc.StartKey)

	r.shMu.state = s.ReplState
	if r.shMu.state.ForceFlushIndex != (roachpb.ForceFlushIndex{}) {
		r.flowControlV2.ForceFlushIndexChangedLocked(context.TODO(), r.shMu.state.ForceFlushIndex.Index)
	}
	// TODO(pav-kv): make a method to initialize the log storage.
	ls := r.asLogStorage()
	ls.shMu.trunc = s.TruncState
	ls.shMu.last = s.LastEntryID

	// Initialize the Raft group. This may replace a Raft group that was installed
	// for the uninitialized replica to process Raft requests or snapshots.
	//
	// We do this before the call to setDescLockedRaftMuLocked(), since it flips
	// isInitialized and we'd like the Raft group to be in place before then.
	if err := r.initRaftGroupRaftMuLockedReplicaMuLocked(); err != nil {
		return err
	}

	r.setDescLockedRaftMuLocked(r.AnnotateCtx(context.TODO()), desc)

	// Only do this if there was a previous lease. This shouldn't be important
	// to do but consider that the first lease which is obtained is back-dated
	// to a zero start timestamp (and this de-flakes some tests). If we set the
	// min proposed TS here, this lease could not be renewed (by the semantics
	// of minLeaseProposedTS); and since minLeaseProposedTS is copied on splits,
	// this problem would multiply to a number of replicas at cluster bootstrap.
	// Instead, we make the first lease special (which is OK) and the problem
	// disappears.
	if r.shMu.state.Lease.Sequence > 0 {
		if waitForPrevLeaseToExpire {
			// Wait for the previous lease to expire. This is important because if the
			// node was restarted, we don't want to reacquire the lease with a start
			// time that overlaps the previous lease.
			// This ensures that we don't serve a write request with the new
			// lease that contradicts a future read served by the old lease before the
			// restart (we would have lost that timestamp cache).
			// Note that we need to sleep (instead of just forwarding the
			// minLeaseProposedTS) because we will run into assertions where the
			// lease proposed time is in the future compared to r.Clock().Now(), and
			// we don't allow acquiring a lease that starts in the future.
			if err := r.waitForPreviousLeaseToExpire(r.store); err != nil {
				return err
			}
		}
		r.mu.minLeaseProposedTS = r.Clock().NowAsClockTimestamp()
	}

	return nil
}

// initRaftGroupRaftMuLockedReplicaMuLocked initializes a Raft group for the
// replica, replacing the existing Raft group if any.
func (r *Replica) initRaftGroupRaftMuLockedReplicaMuLocked() error {
	ctx := r.AnnotateCtx(context.Background())
	rg, err := raft.NewRawNode(newRaftConfig(
		ctx,
		(*replicaRaftStorage)(r),
		raftpb.PeerID(r.replicaID),
		r.shMu.state.RaftAppliedIndex,
		r.store.cfg,
		r.shMu.currentRACv2Mode == rac2.MsgAppPull,
		&raftLogger{ctx: ctx},
		(*replicaRLockedStoreLiveness)(r),
		r.store.raftMetrics,
		r.store.TestingKnobs().RaftTestingKnobs,
	))
	if err != nil {
		return err
	}
	r.mu.internalRaftGroup = rg
	r.mu.raftTracer = *rafttrace.NewRaftTracer(ctx, r.Tracer, r.ClusterSettings(), &r.store.concurrentRaftTraces)
	r.flowControlV2.InitRaftLocked(
		ctx, replica_rac2.NewRaftNode(rg, (*replicaForRACv2)(r)), rg.LogMark())
	return nil
}

// initFromSnapshotLockedRaftMuLocked initializes a Replica when applying the
// initial snapshot.
func (r *Replica) initFromSnapshotLockedRaftMuLocked(
	ctx context.Context, desc *roachpb.RangeDescriptor,
) error {
	if !desc.IsInitialized() {
		return errors.AssertionFailedf("initializing replica with uninitialized desc: %s", desc)
	}

	r.setDescLockedRaftMuLocked(ctx, desc)
	r.setStartKeyLocked(desc.StartKey)
	return nil
}

// IsInitialized is true if we know the metadata of this replica's range, either
// because we created it or we have received an initial snapshot from another
// node. It is false when a replica has been created in response to an incoming
// message but we are waiting for our initial snapshot.
func (r *Replica) IsInitialized() bool {
	return r.isInitialized.Load()
}

// TenantID returns the associated tenant ID and a boolean to indicate that it
// is valid. It will be invalid only if the replica is not initialized.
func (r *Replica) TenantID() (roachpb.TenantID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getTenantIDRLocked()
}

func (r *Replica) getTenantIDRLocked() (roachpb.TenantID, bool) {
	return r.mu.tenantID, r.mu.tenantID != (roachpb.TenantID{})
}

// setDescRaftMuLocked atomically sets the replica's descriptor. It requires raftMu to be
// locked.
func (r *Replica) setDescRaftMuLocked(ctx context.Context, desc *roachpb.RangeDescriptor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setDescLockedRaftMuLocked(ctx, desc)
}

func (r *Replica) setDescLockedRaftMuLocked(ctx context.Context, desc *roachpb.RangeDescriptor) {
	if desc.RangeID != r.RangeID {
		log.Fatalf(ctx, "range descriptor ID (%d) does not match replica's range ID (%d)",
			desc.RangeID, r.RangeID)
	}
	if r.shMu.state.Desc.IsInitialized() &&
		(desc == nil || !desc.IsInitialized()) {
		log.Fatalf(ctx, "cannot replace initialized descriptor with uninitialized one: %+v -> %+v",
			r.shMu.state.Desc, desc)
	}
	if r.shMu.state.Desc.IsInitialized() &&
		!r.shMu.state.Desc.StartKey.Equal(desc.StartKey) {
		log.Fatalf(ctx, "attempted to change replica's start key from %s to %s",
			r.shMu.state.Desc.StartKey, desc.StartKey)
	}

	// NB: It might be nice to assert that the current replica exists in desc
	// however we allow it to not be present for three reasons:
	//
	//   1) When removing the current replica we update the descriptor to the point
	//      of removal even though we will delete the Replica's data in the same
	//      batch. We could avoid setting the local descriptor in this case.
	//   2) When the DisableEagerReplicaRemoval testing knob is enabled. We
	//      could remove all tests which utilize this behavior now that there's
	//      no other mechanism for range state which does not contain the current
	//      store to exist on disk.
	//   3) Various unit tests do not provide a valid descriptor.
	replDesc, found := desc.GetReplicaDescriptor(r.StoreID())
	if found && replDesc.ReplicaID != r.replicaID {
		log.Fatalf(ctx, "attempted to change replica's ID from %d to %d",
			r.replicaID, replDesc.ReplicaID)
	}

	// Initialize the tenant. The must be the first time that the descriptor has
	// been initialized. Note that the desc.StartKey never changes throughout the
	// life of a range.
	if desc.IsInitialized() && r.mu.tenantID == (roachpb.TenantID{}) {
		_, tenantID, err := keys.DecodeTenantPrefix(desc.StartKey.AsRawKey())
		if err != nil {
			log.Fatalf(ctx, "failed to decode tenant prefix from key for "+
				"replica %v: %v", r, err)
		}
		r.mu.tenantID = tenantID
		r.tenantMetricsRef = r.store.metrics.acquireTenant(tenantID)
		if tenantID != roachpb.SystemTenantID {
			r.tenantLimiter = r.store.tenantRateLimiters.GetTenant(ctx, tenantID, r.store.stopper.ShouldQuiesce())
		}
	}

	// Determine if a new replica was added. This is true if the new max replica
	// ID is greater than the old max replica ID.
	oldMaxID := maxReplicaIDOfAny(r.shMu.state.Desc)
	newMaxID := maxReplicaIDOfAny(desc)
	if newMaxID > oldMaxID {
		r.mu.lastReplicaAdded = newMaxID
		r.mu.lastReplicaAddedTime = timeutil.Now()
	} else if r.mu.lastReplicaAdded > newMaxID {
		// The last replica added was removed.
		r.mu.lastReplicaAdded = 0
		r.mu.lastReplicaAddedTime = time.Time{}
	}

	r.rangeStr.store(r.replicaID, desc)
	r.isInitialized.Store(desc.IsInitialized())
	r.connectionClass.set(rpcbase.ConnectionClassForKey(desc.StartKey, defRaftConnClass))
	r.concMgr.OnRangeDescUpdated(desc)
	r.shMu.state.Desc = desc
	r.flowControlV2.OnDescChangedLocked(ctx, desc, r.mu.tenantID)

	// Give the liveness and meta ranges high priority in the Raft scheduler, to
	// avoid head-of-line blocking and high scheduling latency.
	for _, span := range []roachpb.Span{keys.NodeLivenessSpan, keys.MetaSpan} {
		rspan, err := keys.SpanAddr(span)
		if err != nil {
			log.Fatalf(ctx, "can't resolve system span %s: %s", span, err)
		}
		if _, err := desc.RSpan().Intersect(rspan); err == nil {
			r.store.scheduler.AddPriorityID(desc.RangeID)
		}
	}
}

// waitForPreviousLeaseToExpire waits for the previous lease to expire. It does
// so by sleeping until Clock().Now() is in the future of the previous lease
// expiration. This works for expiration-based leases, and leader-leases but
// only best-effort for epoch-based leases since the liveness record might not
// be found in cache after the restart.
func (r *Replica) waitForPreviousLeaseToExpire(store *Store) error {
	st := r.leaseStatusAtRLocked(r.AnnotateCtx(context.TODO()), r.Clock().NowAsClockTimestamp())
	if st.OwnedBy(store.StoreID()) {
		// Only sleep if we were the previous lease owner.
		if err := r.Clock().SleepUntil(
			r.AnnotateCtx(context.TODO()),
			st.Expiration().Next(),
		); err != nil {
			return err
		}
	}
	return nil
}
