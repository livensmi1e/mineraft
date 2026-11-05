package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIndexUsesEnglishCopy(t *testing.T) {
	cluster, err := newCluster([]id{"S1"})
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.close()

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	newWebHandler(cluster).ServeHTTP(response, request)

	body := response.Body.String()
	if !strings.Contains(body, `<html lang="en">`) || !strings.Contains(body, "Click a node to isolate or reconnect it.") {
		t.Fatal("index does not contain the expected English copy")
	}
}

func TestConfigureNetworkRejectsUnknownMode(t *testing.T) {
	cluster, err := newCluster([]id{"S1"})
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.close()

	request := httptest.NewRequest(http.MethodPost, "/api/network", strings.NewReader(`{"mode":"chaos"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	newWebHandler(cluster).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestConfigureNodeChangesNetworkState(t *testing.T) {
	cluster, err := newCluster([]id{"S1"})
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.close()

	request := httptest.NewRequest(http.MethodPost, "/api/nodes", strings.NewReader(`{"id":"S1","enabled":false}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	newWebHandler(cluster).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if cluster.snapshot().Network.Nodes["S1"] {
		t.Fatal("S1 is enabled, want isolated")
	}
}
