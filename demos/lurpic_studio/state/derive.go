package state

import (
	"sort"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
)

func computeFilteredRows(as *AppState) []dataset.Row {
	all := as.Rows.All()

	source := as.SelectedSource.Get()
	var out []dataset.Row
	if source != "" {
		for _, r := range all {
			if r.Region == source {
				out = append(out, r)
			}
		}
	} else {
		out = make([]dataset.Row, len(all))
		copy(out, all)
	}

	agg := as.Aggregation.Get()
	switch agg {
	case AggSum:
		out = aggregateByRegion(out, true)
	case AggAvg:
		out = aggregateByRegion(out, false)
	}
	return out
}

func computeVisibleRows(as *AppState) []dataset.Row {
	filtered := as.FilteredRows.Get()
	page := as.Page.Get()
	start := page * PageSize
	if start >= len(filtered) {
		return []dataset.Row{}
	}
	end := start + PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end]
}

func aggregateByRegion(rows []dataset.Row, sum bool) []dataset.Row {
	type acc struct {
		revenue float64
		users   float64
		count   int
	}
	buckets := make(map[string]*acc)
	order := make([]string, 0, 4)
	for _, r := range rows {
		a, ok := buckets[r.Region]
		if !ok {
			a = &acc{}
			buckets[r.Region] = a
			order = append(order, r.Region)
		}
		a.revenue += r.Revenue
		a.users += r.Users
		a.count++
	}
	out := make([]dataset.Row, 0, len(order))
	for _, reg := range order {
		a := buckets[reg]
		rev := a.revenue
		usr := a.users
		if !sum {
			rev /= float64(a.count)
			usr /= float64(a.count)
		}
		out = append(out, dataset.Row{
			Revenue: rev,
			Users:   usr,
			Region:  reg,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Region < out[j].Region
	})
	return out
}

func computeYDomain(as *AppState) [2]float64 {
	rows := as.FilteredRows.Get()
	if len(rows) == 0 {
		return [2]float64{0, 100}
	}

	var maxVal float64
	for _, r := range rows {
		if r.Revenue > maxVal {
			maxVal = r.Revenue
		}
		if r.Users > maxVal {
			maxVal = r.Users
		}
	}
	if maxVal == 0 {
		return [2]float64{0, 100}
	}

	yMax := as.YAxisMax.Get()
	if yMax > 0 && yMax < maxVal {
		return [2]float64{0, yMax}
	}

	pad := maxVal * 0.1
	return [2]float64{0, maxVal + pad}
}

func computeBarBuckets(as *AppState) []BarBucket {
	rows := as.FilteredRows.Get()
	if len(rows) == 0 {
		return nil
	}
	buckets := make(map[string]*struct{ rev, users float64 })
	order := make([]string, 0, 4)
	for _, r := range rows {
		b, ok := buckets[r.Region]
		if !ok {
			b = &struct{ rev, users float64 }{}
			buckets[r.Region] = b
			order = append(order, r.Region)
		}
		b.rev += r.Revenue
		b.users += r.Users
	}
	out := make([]BarBucket, 0, len(order))
	for _, reg := range order {
		b := buckets[reg]
		out = append(out, BarBucket{
			Region:  reg,
			Revenue: b.rev,
			Users:   b.users,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Region < out[j].Region
	})
	return out
}
