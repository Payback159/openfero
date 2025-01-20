package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No newlines or carriage returns",
			input:    "HelloWorld",
			expected: "HelloWorld",
		},
		{
			name:     "Newline characters",
			input:    "Hello\nWorld",
			expected: "HelloWorld",
		},
		{
			name:     "Carriage return characters",
			input:    "Hello\rWorld",
			expected: "HelloWorld",
		},
		{
			name:     "Newline and carriage return characters",
			input:    "Hello\n\rWorld",
			expected: "HelloWorld",
		},
		{
			name:     "Multiple newline and carriage return characters",
			input:    "\nHello\r\nWorld\r",
			expected: "HelloWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeInput(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeInput(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStringWithCharset(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		charset string
	}{
		{
			name:    "Generate string of length 10 with alphanumeric charset",
			length:  10,
			charset: "abcdefghijklmnopqrstuvwxyz0123456789",
		},
		{
			name:    "Generate string of length 5 with numeric charset",
			length:  5,
			charset: "0123456789",
		},
		{
			name:    "Generate string of length 8 with alphabetic charset",
			length:  8,
			charset: "abcdefghijklmnopqrstuvwxyz",
		},
		{
			name:    "Generate string of length 0",
			length:  0,
			charset: "abcdefghijklmnopqrstuvwxyz0123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringWithCharset(tt.length, tt.charset)
			if len(result) != tt.length {
				t.Errorf("stringWithCharset(%d, %q) = %q; want length %d", tt.length, tt.charset, result, tt.length)
			}

			for _, char := range result {
				if !strings.ContainsRune(tt.charset, char) && tt.charset != "" {
					t.Errorf("stringWithCharset(%d, %q) = %q; contains invalid character %q", tt.length, tt.charset, result, char)
				}
			}
		})
	}
}

// Benchmark the stringWithCharset function
func BenchmarkStringWithCharset(b *testing.B) {
	for i := 0; i < b.N; i++ {
		stringWithCharset(10, "abcdefghijklmnopqrstuvwxyz0123456789")
	}
}

func TestSaveAlert(t *testing.T) {
	tests := []struct {
		name           string
		initialAlerts  []alert
		newAlert       alert
		expectedAlerts []alert
		alertStoreSize int
	}{
		{
			name: "Add alert to non-full store",
			initialAlerts: []alert{
				{Labels: map[string]string{"alertname": "alert1"}},
			},
			newAlert: alert{Labels: map[string]string{"alertname": "alert2"}},
			expectedAlerts: []alert{
				{Labels: map[string]string{"alertname": "alert1"}},
				{Labels: map[string]string{"alertname": "alert2"}},
			},
			alertStoreSize: 10,
		},
		// {
		// 	name: "Add alert to full store",
		// 	initialAlerts: []alert{
		// 		{Labels: map[string]string{"alertname": "alert1"}},
		// 		{Labels: map[string]string{"alertname": "alert2"}},
		// 	},
		// 	newAlert: alert{Labels: map[string]string{"alertname": "alert3"}},
		// 	expectedAlerts: []alert{
		// 		{Labels: map[string]string{"alertname": "alert2"}},
		// 		{Labels: map[string]string{"alertname": "alert3"}},
		// 	},
		// 	alertStoreSize: 2,
		// },
		{
			name:          "Add alert to empty store",
			initialAlerts: []alert{},
			newAlert:      alert{Labels: map[string]string{"alertname": "alert1"}},
			expectedAlerts: []alert{
				{Labels: map[string]string{"alertname": "alert1"}},
			},
			alertStoreSize: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alertStore = make([]alert, 0, tt.alertStoreSize)
			alertStore = append(alertStore, tt.initialAlerts...)
			copy(alertStore, tt.initialAlerts)

			server := &clientsetStruct{}
			server.saveAlert(tt.newAlert)

			if !reflect.DeepEqual(alertStore, tt.expectedAlerts) {
				t.Errorf("saveAlert() = %v; want %v", alertStore, tt.expectedAlerts)
			}
		})
	}
}

func BenchmarkSaveAlert(b *testing.B) {
	initialAlerts := []alert{
		{Labels: map[string]string{"alertname": "alert1"}},
		{Labels: map[string]string{"alertname": "alert2"}},
	}
	newAlert := alert{Labels: map[string]string{"alertname": "alert3"}}
	alertStoreSize := 10

	for i := 0; i < b.N; i++ {
		alertStore = make([]alert, len(initialAlerts), alertStoreSize)
		copy(alertStore, initialAlerts)

		server := &clientsetStruct{}
		server.saveAlert(newAlert)
	}
}

// func TestAlertStoreGetHandler(t *testing.T) {
// 	tests := []struct {
// 		name           string
// 		query          string
// 		expectedStatus int
// 		expectedAlerts []alert
// 	}{
// 		{
// 			name:           "No query parameter",
// 			query:          "",
// 			expectedStatus: http.StatusOK,
// 			expectedAlerts: alertStore,
// 		},
// 		{
// 			name:           "Query parameter matches",
// 			query:          "testAlert",
// 			expectedStatus: http.StatusOK,
// 			expectedAlerts: []alert{
// 				{
// 					Labels: map[string]string{
// 						"alertname": "testAlert",
// 					},
// 					Annotations: map[string]string{
// 						"description": "This is a test alert",
// 					},
// 				},
// 			},
// 		},
// 		{
// 			name:           "Query parameter does not match",
// 			query:          "nonexistentAlert",
// 			expectedStatus: http.StatusOK,
// 			expectedAlerts: []alert{},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			req, err := http.NewRequest("GET", "/alertStore?q="+tt.query, nil)
// 			if err != nil {
// 				t.Fatal(err)
// 			}

// 			rr := httptest.NewRecorder()
// 			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 				server := &clientsetStruct{}
// 				server.alertStoreGetHandler(w, r)
// 			})

// 			handler.ServeHTTP(rr, req)

// 			if status := rr.Code; status != tt.expectedStatus {
// 				t.Errorf("handler returned wrong status code: got %v want %v",
// 					status, tt.expectedStatus)
// 			}

// 			var alerts []alert
// 			err = json.NewDecoder(rr.Body).Decode(&alerts)
// 			if err != nil {
// 				t.Fatal(err)
// 			}

// 			if !reflect.DeepEqual(alerts, tt.expectedAlerts) {
// 				t.Errorf("handler returned unexpected body: got %v want %v",
// 					alerts, tt.expectedAlerts)
// 			}
// 		})
// 	}
// }

func BenchmarkAlertStoreGetHandler(b *testing.B) {
	req, err := http.NewRequest("GET", "/alertStore", nil)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server := &clientsetStruct{}
			server.alertStoreGetHandler(w, r)
		})

		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkAlertStoreGetHandlerWithQuery(b *testing.B) {
	req, err := http.NewRequest("GET", "/alertStore?q=testAlert", nil)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server := &clientsetStruct{}
			server.alertStoreGetHandler(w, r)
		})

		handler.ServeHTTP(rr, req)
	}
}
