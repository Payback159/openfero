package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetAlertsHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/alerts", nil)
	if err != nil {
		t.Fatal(err)
	}
	responserecorder := httptest.NewRecorder()

	(*clientsetStruct).alertsHandler(nil, responserecorder, req)

	if status := responserecorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestSingleAlertPostAlertsHandler(t *testing.T) {

	//TODO: mocking k8s api with testclient
	/*
		https://medium.com/the-phi/mocking-the-kubernetes-client-in-go-for-unit-testing-ddae65c4302
	*/
	jsonFile, err := os.Open("test/singlealert.json")
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()

	req, err := http.NewRequest("POST", "/alerts", jsonFile)
	if err != nil {
		t.Fatal(err)
	}
	responserecorder := httptest.NewRecorder()

	(*clientsetStruct).alertsHandler(nil, responserecorder, req)

	if status := responserecorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func MultipleAlertPostAlertsHandler(t *testing.T) {
	//TODO:
	/*
		Implement a test with alerts.json
	*/
}

func MalformedJSONPostAlertsHandler(t *testing.T) {
	//TODO:
	/*
		Implement test with malformed json
	*/
}
