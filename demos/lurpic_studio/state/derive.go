package state

import (
	"sort"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
)

var regionOrder = []string{"NA", "EU", "APAC", "LATAM"}

func computeVisibleRows(all []dataset.Row, selectedSource string, agg AggMode, page int) []dataset.Row {
	var filtered []dataset.Row
	if selectedSource != "" {
		for _, r := range all {
			if r.Region == selectedSource {
				filtered = append(filtered, r)
			}
		}
	} else {
		filtered = append([]dataset.Row(nil), all...)
	}

	if len(filtered) == 0 {
		return nil
	}

	if agg != AggNone {
		type acc struct {
			revenue float64
			users   float64
			count   int
		}
		aggByRegion := make(map[string]*acc, 4)
		for _, r := range filtered {
			a, ok := aggByRegion[r.Region]
			if !ok {
				aggByRegion[r.Region] = &acc{revenue: r.Revenue, users: r.Users, count: 1}
				continue
			}
			a.revenue += r.Revenue
			a.users += r.Users
			a.count++
		}
		aggregated := make([]dataset.Row, 0, len(aggByRegion))
		for _, region := range regionOrder {
			a, ok := aggByRegion[region]
			if !ok {
				continue
			}
			var rev, usr float64
			if agg == AggAvg {
				rev = a.revenue / float64(a.count)
				usr = a.users / float64(a.count)
			} else {
				rev = a.revenue
				usr = a.users
			}
			aggregated = append(aggregated, dataset.Row{
				Revenue: rev,
				Users:   usr,
				Region:  region,
			})
		}
		filtered = aggregated
	}

	if page < 1 {
		page = 1
	}
	start := (page - 1) * PageSize
	if start >= len(filtered) {
		return nil
	}
	end := start + PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end]
}

func computeYDomain(visibleRows []dataset.Row, yAxisMax float64) [2]float64 {
	if len(visibleRows) == 0 {
		return [2]float64{0, 100}
	}
	max := 0.0
	for _, r := range visibleRows {
		if r.Revenue > max {
			max = r.Revenue
		}
	}
	domainMax := max * 1.1
	if yAxisMax > 0 && domainMax > yAxisMax {
		domainMax = yAxisMax
	}
	return [2]float64{0, domainMax}
}

func computeBarBuckets(visibleRows []dataset.Row) []BarBucket {
	if len(visibleRows) == 0 {
		return nil
	}
	sums := make(map[string]float64, 4)
	for _, r := range visibleRows {
		sums[r.Region] += r.Revenue
	}
	buckets := make([]BarBucket, 0, len(sums))
	for _, region := range regionOrder {
		v, ok := sums[region]
		if !ok {
			continue
		}
		buckets = append(buckets, BarBucket{Region: region, Value: v})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Region < buckets[j].Region
	})
	return buckets
}
