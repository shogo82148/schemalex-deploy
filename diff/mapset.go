package diff

import "sort"

// set is a tiny utils for handling sets.
type set map[string]struct{}

func newSet() set {
	return make(set)
}

func (s set) Add(item string) {
	s[item] = struct{}{}
}

func (s set) Difference(t set) set {
	u := newSet()
	for item := range s {
		if _, ok := t[item]; !ok {
			u[item] = struct{}{}
		}
	}
	return u
}

func (s set) Intersect(t set) set {
	u := newSet()
	if len(s) < len(t) {
		for item := range s {
			if _, ok := t[item]; ok {
				u[item] = struct{}{}
			}
		}
	} else {
		for item := range t {
			if _, ok := s[item]; ok {
				u[item] = struct{}{}
			}
		}
	}
	return u
}

func (s set) ToSlice() []string {
	ret := make([]string, 0, len(s))
	for item := range s {
		ret = append(ret, item)
	}
	sort.Strings(ret)
	return ret
}

func (s set) Cardinality() int {
	return len(s)
}
