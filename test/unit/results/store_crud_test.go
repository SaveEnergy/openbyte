package results_test

import (
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/results"
)

func TestStoreSaveAndGet(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	r := results.Result{
		DownloadMbps:     123.4,
		UploadMbps:       56.7,
		LatencyMs:        12.3,
		JitterMs:         1.5,
		LoadedLatencyMs:  25.0,
		BufferbloatGrade: "A",
		IPv4:             "1.2.3.4",
		IPv6:             "::1",
		ServerName:       "Test Server",
	}

	id, err := store.Save(r)
	if err != nil {
		t.Fatalf(storeSaveFmt, err)
	}
	if len(id) != 8 {
		t.Fatalf(storeExpectedIDLenFmt, id)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf(storeGetFmt, err)
	}
	if got == nil {
		t.Fatal(storeGetReturnedNilMsg)
	}
	if got.DownloadMbps != 123.4 {
		t.Errorf(storeDownloadMbpsFmt, got.DownloadMbps)
	}
	if got.UploadMbps != 56.7 {
		t.Errorf(storeUploadMbpsFmt, got.UploadMbps)
	}
	if got.IPv4 != "1.2.3.4" {
		t.Errorf(storeIPv4Fmt, got.IPv4)
	}
	if got.BufferbloatGrade != "A" {
		t.Errorf(storeBufferbloatFmt, got.BufferbloatGrade)
	}
}

func TestStoreGetNotFound(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	got, err := store.Get("abcd1234")
	if err != nil {
		t.Fatalf(storeGetFmt, err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing ID, got %+v", got)
	}
}

func TestStoreCloseIdempotent(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	store.Close()
	store.Close()
}

func TestGenerateIDUsesValidCharset(t *testing.T) {
	store, cleanup := tempStore(t, 10000)
	defer cleanup()

	const samples = 2000
	const idCharset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	seen := make(map[rune]struct{})

	for range samples {
		id, err := store.Save(results.Result{
			DownloadMbps: 1, UploadMbps: 1, LatencyMs: 1, JitterMs: 1,
		})
		if err != nil {
			t.Fatalf(storeSaveResultFmt, err)
		}
		for _, ch := range id {
			if !strings.ContainsRune(idCharset, ch) {
				t.Fatalf("id has invalid char %q in %q", ch, id)
			}
			seen[ch] = struct{}{}
		}
	}

	if len(seen) != len(idCharset) {
		t.Fatalf("seen charset size = %d, want %d", len(seen), len(idCharset))
	}
}
