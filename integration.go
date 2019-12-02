// +build integration

package main

// func stockpriceWebUI(w http.ResponseWriter, r *http.Request) {
// 	// log.Println("heavy process starts")
// 	// time.Sleep(3 * time.Second)
// 	// log.Println("done")
// 	// w.Header().Set("Content-Type", "text/plain")
// 	// w.Write([]byte("hello\n"))

// 	path := r.URL.Path                  // クエリの中身を取得
// 	code := strings.TrimLeft(path, "/") // 「/」を削除
// 	content, err := ioutil.ReadFile(fmt.Sprintf("testdata/%s.html", code))
// 	if err != nil {
// 		log.Printf("failed to read testfile: %v", err)
// 	}
// 	fmt.Fprintf(w, string(content))
// }

// // var (
// // 	simpleHTTPServer      http.Server
// // 	sigChan               chan os.Signal
// // 	simpleServiceShutdown chan bool
// // )

// // func handler(w http.ResponseWriter, r *http.Request) {
// // 	// log.Println("heavy process starts")
// // 	// time.Sleep(5 * time.Second)
// // 	// log.Println("done")
// // 	w.Header().Set("Content-Type", "text/plain")
// // 	w.Write([]byte("hello\n"))
// // }

// // func handler(w http.ResponseWriter, r *http.Request) {
// // 	fmt.Fprint(w, "Hello")
// // }

// func server(done chan interface{}, srv *http.Server) chan error {
// 	errCh := make(chan error)
// 	go func() {
// 		defer close(errCh)
// 		fmt.Println("starting http server on :8081")
// 		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
// 			close(done)
// 			errCh <- fmt.Errorf("Server closed with error: %v", err)
// 			return
// 		}
// 		//log.Println("Server closed successfully")
// 	}()
// 	return errCh
// }

// func doneServer(ctx context.Context, srv *http.Server, done chan interface{}) chan error {
// 	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
// 	defer cancel()

// 	errCh := make(chan error)
// 	go func() {
// 		defer close(errCh)
// 		<-done
// 		if err := srv.Shutdown(ctx); err != nil {
// 			errCh <- fmt.Errorf("Failed to gracefully shutdown: %v", err)
// 			return
// 		}
// 		log.Println("Server shutdown gracefully")
// 	}()
// 	return errCh
// }

// func remoteHello(domain string) string {
// 	// /greetingにクエリパラメータgreet=Helloを渡してGet問い合わせする
// 	res, err := http.Get(domain + "/greeting?greet=Hello")

// 	// エラー処理
// 	if err != nil {
// 		fmt.Println("Error")
// 		return "error"
// 	}
// 	defer res.Body.Close()

// 	// レスポンスを戻り値にする
// 	res_str, _ := ioutil.ReadAll(res.Body)
// 	return string(res_str)
// }

// func stock(domain string) string {
// 	// /greetingにクエリパラメータgreet=Helloを渡してGet問い合わせする
// 	res, err := http.Get(domain + "?scode=1802")

// 	// エラー処理
// 	if err != nil {
// 		fmt.Println("Error")
// 		return "error"
// 	}
// 	defer res.Body.Close()

// 	// レスポンスを戻り値にする
// 	res_str, _ := ioutil.ReadAll(res.Body)
// 	return string(res_str)
// }
