// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

import { Location } from "history";
import { match as Match } from "react-router-dom";

/*
 * parseSplatParams function returns remaining part of the path
 * after matched part.
 * ```
 * For example:
 * match.path: `overview/map`
 * location.path: `overview/map/region=us-west/zone=a`
 * result: region=us-west/zone=a
 * ```
 */
export function parseSplatParams(match: Match, location: Location) {
  let splat = location.pathname.replace(`${match.path}`, "");
  if (splat.startsWith("/")) {
    splat = splat.slice(1);
  }
  return splat;
}
