package main

import (
	"errors"
	"log"
	"net/http"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cluster, err := newCluster([]id{"S1", "S2", "S3", "S4", "S5"})
	if err != nil {
		return err
	}
	defer cluster.close()

	server := &http.Server{
		Addr:              ":8080",
		Handler:           newWebHandler(cluster),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Raft lab is available at http://localhost%s", server.Addr)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
