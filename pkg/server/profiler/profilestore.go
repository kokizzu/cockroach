// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package profiler

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/server/dumpstore"
	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
)

var maxProfiles = settings.RegisterIntSetting(
	settings.ApplicationLevel,
	"server.mem_profile.max_profiles",
	"maximum number of profiles to be kept per ramp-up of memory usage. "+
		"A ramp-up is defined as a sequence of profiles with increasing usage.",
	5,
)

func init() {
	_ = settings.RegisterIntSetting(
		settings.ApplicationLevel,
		"server.heap_profile.max_profiles", "use server.mem_profile.max_profiles instead", 5,
		settings.Retired)
}

// profileStore represents the directory where heap profiles are stored.
// It supports automatic garbage collection of old profiles.
type profileStore struct {
	*dumpstore.DumpStore
	prefix string
	suffix string
	st     *cluster.Settings
}

func newProfileStore(
	store *dumpstore.DumpStore, prefix, suffix string, st *cluster.Settings,
) *profileStore {
	s := &profileStore{DumpStore: store, prefix: prefix, suffix: suffix, st: st}
	return s
}

func (s *profileStore) gcProfiles(ctx context.Context, now time.Time) {
	s.GC(ctx, now, s)
}

func (s *profileStore) makeNewFileName(timestamp time.Time, curHeap int64) string {
	// We place the timestamp immediately after the (immutable) file
	// prefix to ensure that a directory listing sort also sorts the
	// profiles in timestamp order.
	fileName := fmt.Sprintf("%s.%s.%d%s",
		s.prefix, timestamp.Format(timestampFormat), curHeap, s.suffix)
	return s.GetFullPath(fileName)
}

// PreFilter is part of the dumpstore.Dumper interface.
func (s *profileStore) PreFilter(
	ctx context.Context, files []os.FileInfo, cleanupFn func(fileName string) error,
) (preserved map[int]bool, _ error) {
	maxP := maxProfiles.Get(&s.st.SV)
	preserved = s.cleanupLastRampup(ctx, files, maxP, cleanupFn)
	return
}

// CheckOwnsFile is part of the dumpstore.Dumper interface.
func (s *profileStore) CheckOwnsFile(ctx context.Context, fi os.FileInfo) bool {
	ok, _, _ := s.parseFileName(ctx, fi.Name())
	return ok
}

// cleanupLastRampup parses the filenames in files to detect the
// last ramp-up (sequence of increasing heap usage). If there
// are more than maxD entries in the last ramp-up, the fn closure
// is called for each of them.
//
// files is assumed to be sorted in chronological order already,
// oldest entry first.
//
// The preserved return value contains the indexes in files
// corresponding to the last ramp-up that were not passed to fn.
func (s *profileStore) cleanupLastRampup(
	ctx context.Context, files []os.FileInfo, maxP int64, fn func(string) error,
) (preserved map[int]bool) {
	preserved = make(map[int]bool)
	curMaxHeap := uint64(math.MaxUint64)
	numFiles := int64(0)
	for i := len(files) - 1; i >= 0; i-- {
		ok, _, curHeap := s.parseFileName(ctx, files[i].Name())
		if !ok {
			continue
		}

		if curHeap > curMaxHeap {
			// This is the end of a ramp-up sequence. We're done.
			break
		}

		// Keep the currently seen heap for the next iteration.
		curMaxHeap = curHeap

		// We saw one file.
		numFiles++

		// Did we encounter the maximum?
		if numFiles > maxP {
			// Yes: clean this up.
			if err := fn(files[i].Name()); err != nil {
				log.Warningf(ctx, "%v", err)
			}
		} else {
			// No: we preserve this file.
			preserved[i] = true
		}
	}

	return preserved
}

// parseFileName retrieves the components of a file name generated by makeNewFileName().
func (s *profileStore) parseFileName(
	ctx context.Context, fileName string,
) (ok bool, timestamp time.Time, heapUsage uint64) {
	parts := strings.Split(fileName, ".")
	numParts := 4 /* prefix, date/time, milliseconds,  heap usage */
	if len(parts) < numParts || parts[0] != s.prefix {
		// Not for us. Silently ignore.
		return
	}
	maybeTimestamp := parts[1] + "." + parts[2]
	var err error
	timestamp, err = time.Parse(timestampFormat, maybeTimestamp)
	if err != nil {
		log.Warningf(ctx, "%v", errors.Wrapf(err, "%s", fileName))
		return
	}
	heapUsage, err = strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		log.Warningf(ctx, "%v", errors.Wrapf(err, "%s", fileName))
		return
	}
	ok = true
	return
}
