package main

import "math"

// Scalar fields retain the latest observation for old clients. Summary values
// describe all reports in the bucket; averages are observation-weighted, not
// time-weighted, and no percentile is inferred from these bounded aggregates.
type sampleValues struct {
	CPU      float64 `json:"cpu"`
	Memory   float64 `json:"memory"`
	Incoming float64 `json:"incoming"`
	Outgoing float64 `json:"outgoing"`
	Read     float64 `json:"read"`
	Write    float64 `json:"write"`
}

type sampleSummary struct {
	Count uint64       `json:"count"`
	Start string       `json:"start"`
	End   string       `json:"end"`
	Min   sampleValues `json:"min"`
	Max   sampleValues `json:"max"`
	Avg   sampleValues `json:"avg"`
}

func valuesOf(v sample) sampleValues {
	return sampleValues{v.CPU, v.Memory, v.Incoming, v.Outgoing, v.Read, v.Write}
}

func summaryOf(v sample) sampleSummary {
	if v.Summary != nil && v.Summary.Count > 0 {
		return *v.Summary
	}
	values := valuesOf(v)
	return sampleSummary{Count: 1, Start: v.At, End: v.At, Min: values, Max: values, Avg: values}
}

func combineValues(a, b sampleValues, f func(float64, float64) float64) sampleValues {
	return sampleValues{f(a.CPU, b.CPU), f(a.Memory, b.Memory), f(a.Incoming, b.Incoming), f(a.Outgoing, b.Outgoing), f(a.Read, b.Read), f(a.Write, b.Write)}
}

func mergeSamples(a, b sample) sample {
	x, y := summaryOf(a), summaryOf(b)
	count := x.Count + y.Count
	weight := float64(y.Count) / float64(count)
	avg := combineValues(x.Avg, y.Avg, func(old, next float64) float64 { return old + (next-old)*weight })
	b.At = a.At
	b.Summary = &sampleSummary{Count: count, Start: x.Start, End: y.End, Min: combineValues(x.Min, y.Min, math.Min), Max: combineValues(x.Max, y.Max, math.Max), Avg: avg}
	return b
}

func reduceSamples(values []sample, limit int) []sample {
	if len(values) == 0 {
		return []sample{}
	}
	if limit > len(values) {
		limit = len(values)
	}
	out := make([]sample, 0, limit)
	for i := 0; i < limit; i++ {
		start, end := i*len(values)/limit, (i+1)*len(values)/limit
		bucket := values[start]
		initial := summaryOf(bucket)
		bucket.Summary = &initial
		for _, v := range values[start+1 : end] {
			bucket = mergeSamples(bucket, v)
		}
		out = append(out, bucket)
	}
	return out
}
