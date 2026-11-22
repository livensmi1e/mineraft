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
	if !strings.Contains(body, `<html lang="en">`) || !strings.Contains(body, "Click a node to inspect it.") {
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

func TestNewClusterStartsInReliableNetworkMode(t *testing.T) {
	cluster, err := newCluster([]id{"S1"})
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.close()

	if cluster.snapshot().Network.Mode != reliableNetwork {
		t.Fatalf("network mode = %q, want %q", cluster.snapshot().Network.Mode, reliableNetwork)
	}
}

func TestSubmitCommandAppendsToLeader(t *testing.T) {
	cluster, err := newCluster([]id{"S1"})
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.close()

	leader := cluster.servers["S1"]
	leader.mu.Lock()
	leader.currentTerm = 1
	leader.becomeLeaderLocked()
	leader.mu.Unlock()

	request := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(`{"command":"set x=1"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	newWebHandler(cluster).ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	log := leader.snapshot().Log
	if len(log) != 1 {
		t.Fatal("leader did not append the command")
	}
	if log[0] != (logEntry{Command: "set x=1", Term: 1}) {
		t.Fatalf("log entry = %+v, want command and term", log[0])
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
