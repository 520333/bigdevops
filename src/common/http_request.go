package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"go.uber.org/zap"
)

func PostWithData(logger *zap.Logger, funcName string, tw int, url string, data interface{}) ([]byte, error) {
	client := &http.Client{}
	bytesData, err := json.Marshal(data)
	if err != nil {
		logger.Error(fmt.Sprintf("[HttpPostPushDataJsonMarshalError][funcName:%v][url:%v][err:%v]", funcName, url, err))
		return nil, err
	}
	reader := bytes.NewReader(bytesData)
	client.Timeout = time.Duration(tw) * time.Second
	req, err := http.NewRequest("POST", url, reader)
	if err != nil {
		logger.Error(fmt.Sprintf("[PostWithBearerToken.http.NewRequest.error][funcName:%v][url:%v][err:%v]", funcName, url, err))
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error(fmt.Sprintf("[PostWithBearerToken.request.error][funcName:%v][url:%v][err:%v]", funcName, url, err))
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		logger.Error(fmt.Sprintf("[PostWithBearerToken.StatusCode.not2xx.error][funcName:%+v][url:%v][code:%v][resp_body_text:%v][err:%v]", funcName, url, resp.StatusCode, string(bodyBytes), err))
		return nil, errors.New(fmt.Sprintf("server returned HTTP status %s", resp.Status))
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error(fmt.Sprintf("[PostWithBearerToken.readbody.error][funcName:%+v][url:%v][err:%v]", funcName, url, err))
		return nil, err
	}
	return bodyBytes, err
}

func PostWithJsonString(logger *zap.Logger, funcName string, tw int, url string, jsonStr string, paramsMap map[string]string, headersMap map[string]string) ([]byte, error) {
	client := &http.Client{}
	bytesData := []byte(jsonStr)
	reader := bytes.NewBuffer(bytesData)
	client.Timeout = time.Duration(tw) * time.Second
	req, err := http.NewRequest("POST", url, reader)
	if err != nil {
		logger.Error(fmt.Sprintf("[PostWithBearerToken.http.NewRequest.error][funcName:%+v][url:%v][err:%v]", funcName, url, err))
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// params
	q := req.URL.Query()
	for k, v := range paramsMap {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	// headers
	for k, v := range headersMap {
		req.Header.Add(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error(fmt.Sprintf("[PostWithBearerToken.request.error][funcName:%+v][url:%v][err:%v]", funcName, url, err))
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		logger.Error(fmt.Sprintf("[PostWithBearerToken.StatusCode.not2xx.error][funcName:%+v][url:%v][code:%v][resp_body_text:%v][err:%v]", funcName, url, resp.StatusCode, string(bodyBytes), err))
		return nil, errors.New(fmt.Sprintf("server returned HTTP status %s", resp.Status))
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error(fmt.Sprintf("[PostWithBearerToken.readbody.error][funcName:%+v][url:%v][err:%v]", funcName, url, err))
		return nil, err
	}
	return bodyBytes, err
}

func DeleteWithId(logger *zap.Logger, funcName string, tw int, url string, paramsMap map[string]string, headersMap map[string]string) ([]byte, error) {
	client := &http.Client{}
	bytesData := []byte("")
	reader := bytes.NewBuffer(bytesData)
	client.Timeout = time.Duration(tw) * time.Second
	req, err := http.NewRequest("DELETE", url, reader)
	if err != nil {
		logger.Error(fmt.Sprintf("[DeleteWithId.http.NewRequest.error][funcName:%+v][url:%v][err:%v]", funcName, url, err))
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// params
	q := req.URL.Query()
	for k, v := range paramsMap {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	// headers
	for k, v := range headersMap {
		req.Header.Add(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error(fmt.Sprintf("[DeleteWithId.request.error][funcName:%+v][url:%v][err:%v]", funcName, url, err))
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		logger.Error(fmt.Sprintf("[DeleteWithId.StatusCode.not2xx.error][funcName:%+v][url:%v][code:%v][resp_body_text:%v][err:%v]", funcName, url, resp.StatusCode, string(bodyBytes), err))
		return nil, errors.New(fmt.Sprintf("server returned HTTP status %s", resp.Status))
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error(fmt.Sprintf("[DeleteWithId.readbody.error][funcName:%+v][url:%v][err:%v]", funcName, url, err))
		return nil, err
	}
	return bodyBytes, err
}
