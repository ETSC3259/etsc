// Copyright 2015 The go-etsc Authors
// This file is part of the go-etsc library.
//
// The go-etsc library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-etsc library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-etsc library. If not, see <http://www.gnu.org/licenses/>.

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/ETSC3259/etsc/metrics"
)

var (
	headerInMeter      = metrics.NewRegisteredMeter("etsc/downloader/headers/in", nil)
	headerReqTimer     = metrics.NewRegisteredTimer("etsc/downloader/headers/req", nil)
	headerDropMeter    = metrics.NewRegisteredMeter("etsc/downloader/headers/drop", nil)
	headerTimeoutMeter = metrics.NewRegisteredMeter("etsc/downloader/headers/timeout", nil)

	bodyInMeter      = metrics.NewRegisteredMeter("etsc/downloader/bodies/in", nil)
	bodyReqTimer     = metrics.NewRegisteredTimer("etsc/downloader/bodies/req", nil)
	bodyDropMeter    = metrics.NewRegisteredMeter("etsc/downloader/bodies/drop", nil)
	bodyTimeoutMeter = metrics.NewRegisteredMeter("etsc/downloader/bodies/timeout", nil)

	receiptInMeter      = metrics.NewRegisteredMeter("etsc/downloader/receipts/in", nil)
	receiptReqTimer     = metrics.NewRegisteredTimer("etsc/downloader/receipts/req", nil)
	receiptDropMeter    = metrics.NewRegisteredMeter("etsc/downloader/receipts/drop", nil)
	receiptTimeoutMeter = metrics.NewRegisteredMeter("etsc/downloader/receipts/timeout", nil)

	stateInMeter   = metrics.NewRegisteredMeter("etsc/downloader/states/in", nil)
	stateDropMeter = metrics.NewRegisteredMeter("etsc/downloader/states/drop", nil)
)
