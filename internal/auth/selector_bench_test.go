package auth

import (
	"fmt"
	"testing"
)

func benchAccountSlice(n int) []*Account {
	out := make([]*Account, n)
	for i := 0; i < n; i++ {
		out[i] = &Account{
			FilePath: fmt.Sprintf("p%d.json", i),
		}
		out[i].atomicStatus.Store(int32(StatusActive))
	}
	return out
}

func BenchmarkFilterAvailable10k(b *testing.B) {
	accs := benchAccountSlice(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filterAvailable("gpt-5", accs)
	}
}

func BenchmarkRoundRobinGetOrRefreshCache10k(b *testing.B) {
	accs := benchAccountSlice(10000)
	s := NewRoundRobinSelector()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.getOrRefreshCache(accs)
	}
}

func BenchmarkRoundRobinPick10k(b *testing.B) {
	accs := benchAccountSlice(10000)
	s := NewRoundRobinSelector()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Pick("gpt-5", accs)
	}
}
