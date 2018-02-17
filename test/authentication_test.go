package test

import (
	"testing"
	"os"
	"fmt"
	"time"
	"sync"
	"context"

	webwire "github.com/qbeon/webwire-go"
	webwire_client "github.com/qbeon/webwire-go/client"
)

// TestAuthentication verifies the server is connectable,
// and is able to receives requests and signals, create sessions
// and identify clients during request- and signal handling
func TestAuthentication(t *testing.T) {
	var clientSignal sync.WaitGroup
	clientSignal.Add(1)
	var createdSession *webwire.Session
	expectedCredentials := []byte("secret_credentials")
	expectedConfirmation := []byte("session_is_correct")
	currentStep := 1

	// Initialize webwire server
	server := setupServer(
		t,
		nil,
		// onSignal
		func(ctx context.Context) {
			defer clientSignal.Done()
			// Extract request message and requesting client from the context
			msg := ctx.Value(webwire.MESSAGE).(webwire.Message)
			compareSessions(t, createdSession, msg.Client.Session)
		},
		// onRequest
		func(ctx context.Context) ([]byte, *webwire.Error) {
			// Extract request message and requesting client from the context
			msg := ctx.Value(webwire.MESSAGE).(webwire.Message)

			// If already authenticated then check session
			if currentStep > 1 {
				compareSessions(t, createdSession, msg.Client.Session)
				return expectedConfirmation, nil
			}

			// Create a new session
			newSession := webwire.NewSession(
				webwire.Os_UNKNOWN,
				"user agent",
				nil,
			)
			createdSession = &newSession

			// Try to register the newly created session and bind it to the client
			if err := msg.Client.CreateSession(createdSession); err != nil {
				return nil, &webwire.Error {
					"INTERNAL_ERROR",
					fmt.Sprintf("Internal server error: %s", err),
				}
			}

			// Authentication step is passed
			currentStep = 2

			// Return the key of the newly created session
			return []byte(createdSession.Key), nil
		},
		// OnSaveSession
		func(session *webwire.Session) error {
			// Verify the session
			compareSessions(t, createdSession, session)
			return nil
		},
		// OnFindSession
		func(_ string) (*webwire.Session, error) {
			return nil, nil
		},
		// OnSessionClosure
		func(_ string) error {
			return nil
		},
		nil,
	)
	go server.Run()

	// Initialize client
	client := webwire_client.NewClient(
		server.Addr,
		nil,
		nil,
		5 * time.Second,
		os.Stdout,
		os.Stderr,
	)
	defer client.Close()

	// Send authentication request and await reply
	authReqReply, err := client.Request(expectedCredentials)
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}

	// Verify reply
	comparePayload(t, "authentication reply", []byte(createdSession.Key), authReqReply)


	// Send a test-request to verify the session on the server and await response
	testReqReply, err := client.Request(expectedCredentials)
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}

	// Verify reply
	comparePayload(t, "test reply", expectedConfirmation, testReqReply)

	// Send a test-signal to verify the session on the server
	if err := client.Signal(expectedCredentials); err != nil {
		t.Fatalf("Request failed: %s", err)
	}
	clientSignal.Wait()
}