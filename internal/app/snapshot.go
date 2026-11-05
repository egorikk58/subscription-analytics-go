package app

import (
	"time"

	"subscription-analytics/internal/core"
)

func preparePublishedSnapshot(d core.Dataset, seedConfig string, polledAt time.Time) (core.Snapshot, error) {
	var config core.DatasetInfo
	if err := config.SetConfig(seedConfig); err != nil {
		return core.Snapshot{}, err
	}

	// A poll of static mocks does not extend their observation window. A real
	// event can extend it; importing a metadata correction or a future billing
	// boundary cannot. Keep the poll timestamp for snapshot and HTTP metadata.
	observedThrough := config.DatasetAsOf.UTC()
	if observedThrough.After(polledAt) {
		observedThrough = polledAt
	}
	include := func(event time.Time) {
		if event.After(observedThrough) && !event.After(polledAt) {
			observedThrough = event.UTC()
		}
	}
	for _, user := range d.Users {
		include(user.CreatedAt)
	}
	for _, sub := range d.Subscriptions {
		include(sub.StartedAt)
		if sub.CanceledAt != nil {
			include(*sub.CanceledAt)
		}
	}
	for _, payment := range d.Payments {
		include(payment.CreatedAt)
	}

	snapshot, err := core.Prepare(d, observedThrough)
	if err != nil {
		return core.Snapshot{}, err
	}
	snapshot.DataAsOf = polledAt
	snapshot.Dataset.Seed = config.Seed
	snapshot.Dataset.GeneratorVersion = config.GeneratorVersion
	snapshot.Dataset.HistoryMonths = config.HistoryMonths
	snapshot.Dataset.DatasetAsOf = config.DatasetAsOf
	return snapshot, nil
}
