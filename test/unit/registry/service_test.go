package registry_test

import (
	"sync"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/registry"
	"github.com/saveenergy/openbyte/pkg/types"
)

const (
	serverIDPrimary   = "s1"
	serverIDSecondary = "s2"
	serverIDTertiary  = "s3"
	serverMissingID   = "missing"
	serverNameOne     = "Server 1"
	serverNameBefore  = "Before"
	serverNameAfter   = "After"
)

func testInfo(id, name string) types.ServerInfo {
	return types.ServerInfo{
		ID:       id,
		Name:     name,
		Host:     "localhost",
		TCPPort:  8081,
		UDPPort:  8082,
		Health:   "healthy",
		MaxTests: 10,
	}
}

func TestServiceRegisterAndGet(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)

	info := testInfo(serverIDPrimary, serverNameOne)
	svc.Register(info)

	got, ok := svc.Get(serverIDPrimary)
	if !ok {
		t.Fatal("expected server to exist after Register")
	}
	if got.Name != serverNameOne {
		t.Errorf("name = %q, want %q", got.Name, serverNameOne)
	}
	if got.LastSeen.IsZero() {
		t.Error("LastSeen should be set")
	}
	if got.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set")
	}
}

func TestServiceGetNotFound(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)

	_, ok := svc.Get(serverMissingID)
	if ok {
		t.Fatal("expected false for nonexistent server")
	}
}

func TestServiceGetReturnsCopy(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)
	svc.Register(testInfo(serverIDPrimary, "Original"))

	got, _ := svc.Get(serverIDPrimary)
	got.Name = "Modified"

	got2, _ := svc.Get(serverIDPrimary)
	if got2.Name != "Original" {
		t.Errorf("Get should return a copy; name = %q, want Original", got2.Name)
	}
}

func TestServiceUpdate(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)
	svc.Register(testInfo(serverIDPrimary, serverNameBefore))

	updated := testInfo(serverIDPrimary, serverNameAfter)
	updated.MaxTests = 99
	if !svc.Update(serverIDPrimary, updated) {
		t.Fatal("Update should return true for existing server")
	}

	got, _ := svc.Get(serverIDPrimary)
	if got.Name != serverNameAfter {
		t.Errorf("name = %q, want %s", got.Name, serverNameAfter)
	}
	if got.MaxTests != 99 {
		t.Errorf("max_tests = %d, want 99", got.MaxTests)
	}
}

func TestServiceUpdateNotFound(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)

	if svc.Update(serverMissingID, testInfo(serverMissingID, "X")) {
		t.Fatal("Update should return false for missing server")
	}
}

func TestServiceDeregister(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)
	svc.Register(testInfo(serverIDPrimary, serverNameOne))

	if !svc.Deregister(serverIDPrimary) {
		t.Fatal("Deregister should return true for existing server")
	}
	if svc.Count() != 0 {
		t.Errorf("count = %d, want 0 after deregister", svc.Count())
	}
}

func TestServiceDeregisterNotFound(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)

	if svc.Deregister(serverMissingID) {
		t.Fatal("Deregister should return false for missing server")
	}
}

func TestServiceList(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)
	svc.Register(testInfo(serverIDPrimary, "A"))
	svc.Register(testInfo(serverIDSecondary, "B"))
	svc.Register(testInfo(serverIDTertiary, "C"))

	list := svc.List()
	if len(list) != 3 {
		t.Fatalf("List len = %d, want 3", len(list))
	}
}

func TestServiceListEmpty(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)
	list := svc.List()
	if len(list) != 0 {
		t.Fatalf("List len = %d, want 0", len(list))
	}
}

func TestServiceListHealthy(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)

	healthy := testInfo(serverIDPrimary, "Healthy")
	healthy.Health = "healthy"
	svc.Register(healthy)

	unhealthy := testInfo(serverIDSecondary, "Unhealthy")
	unhealthy.Health = "degraded"
	svc.Register(unhealthy)

	list := svc.ListHealthy()
	if len(list) != 1 {
		t.Fatalf("ListHealthy len = %d, want 1", len(list))
	}
	if list[0].ID != serverIDPrimary {
		t.Errorf("healthy server ID = %q, want %s", list[0].ID, serverIDPrimary)
	}
}

func TestServiceCount(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)
	if svc.Count() != 0 {
		t.Fatalf("empty count = %d, want 0", svc.Count())
	}

	svc.Register(testInfo(serverIDPrimary, "A"))
	svc.Register(testInfo(serverIDSecondary, "B"))
	if svc.Count() != 2 {
		t.Fatalf("count = %d, want 2", svc.Count())
	}
}

func TestServiceStartStop(t *testing.T) {
	svc := registry.NewService(30*time.Second, 50*time.Millisecond)
	svc.Start()
	svc.Stop()
	// Double stop should be safe (sync.Once)
	svc.Stop()
}

func TestServiceStartIdempotent(t *testing.T) {
	svc := registry.NewService(30*time.Second, 20*time.Millisecond)
	svc.Start()
	svc.Start()
	time.Sleep(30 * time.Millisecond)
	svc.Stop()
}

func TestServiceCleanupExpired(t *testing.T) {
	// TTL of 50ms + short cleanup interval
	svc := registry.NewService(50*time.Millisecond, 20*time.Millisecond)
	svc.Register(testInfo(serverIDPrimary, "Expiring"))
	svc.Start()

	// Wait for cleanup to fire
	time.Sleep(150 * time.Millisecond)
	svc.Stop()

	if svc.Count() != 0 {
		t.Errorf("expired server should be cleaned up; count = %d", svc.Count())
	}
}

func TestServiceDefaultTTL(t *testing.T) {
	svc := registry.NewService(0, 0)
	svc.Register(testInfo(serverIDPrimary, "Default"))
	got, ok := svc.Get(serverIDPrimary)
	if !ok {
		t.Fatal("expected server to exist")
	}
	// Default TTL should be 60s, so expiry is ~60s from now
	if time.Until(got.ExpiresAt) < 55*time.Second {
		t.Errorf("default TTL too short: expires in %v", time.Until(got.ExpiresAt))
	}
}

func TestServiceConcurrentAccess(t *testing.T) {
	svc := registry.NewService(30*time.Second, 10*time.Second)
	svc.Start()
	defer svc.Stop()

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "s" + string(rune('A'+n%26))
			svc.Register(testInfo(id, "Concurrent"))
			svc.Get(id)
			svc.List()
			svc.ListHealthy()
			svc.Count()
		}(i)
	}
	wg.Wait()
}
