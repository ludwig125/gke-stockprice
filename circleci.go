package main

import (
	"bytes"
	"fmt"
	"net/http"
)

func requestCircleci(token, job string) error {
	// 参考
	// https://circleci.com/docs/ja/2.0/api-job-trigger/
	// https://circleci.com/docs/api/#trigger-a-new-job
	client := &http.Client{}
	circleciURL := "https://circleci.com/api/v1.1/project/github/ludwig125/gke-stockprice/tree/master"
	j := fmt.Sprintf(`{"build_parameters": {"CIRCLE_JOB": "%s"}}`, job)
	req, err := http.NewRequest("POST", circleciURL, bytes.NewBuffer([]byte(j)))
	req.SetBasicAuth(token, "")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// circleci APIを呼び出すと、201 Created が返ってくるのでチェック
	if resp.StatusCode != 201 {
		return fmt.Errorf("status code Error. %v", resp.StatusCode)
	}

	// レスポンス本文が見たい場合はここのコメントアウトを外す
	// body, err := ioutil.ReadAll(resp.Body)
	// fmt.Println(string(body))
	return nil
}
