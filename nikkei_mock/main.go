package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var scodeList = []string{"1802", "2587", "3382", "4684", "5105", "6506", "6758", "7201", "8058", "9432"}

func nikkeiMock(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("scode")
	if code == "" {
		fmt.Fprint(w, "failed to get scode")
		return
	}
	// testdataにない銘柄コードをリクエストされたら終了
	hasCode := func(code string) bool {
		for _, c := range scodeList {
			if code == c {
				return true
			}
		}
		return false
	}
	if !hasCode(code) {
		fmt.Fprint(w, "no match code in testdata")
		return
	}
	content, err := ioutil.ReadFile(fmt.Sprintf("/go/bin/codes/%s.html", code))
	if err != nil {
		log.Printf("failed to read testdata: %v", err)
	}
	// レスポンスを設定する
	w.Header().Set("content-Type", "text")
	fmt.Fprintf(w, string(content))
}

func main() {
	srv := &http.Server{
		Addr:    ":8181",
		Handler: http.HandlerFunc(nikkeiMock),
	}

	go func() {
		log.Println("starting HTTP server: 8181")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("ERROR shutdown:", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Print("Server Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Println("ERROR shutdown:", err)
	}
	log.Print("Server Stopped successfully")
}
