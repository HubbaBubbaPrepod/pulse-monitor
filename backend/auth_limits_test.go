package main

import (
	"fmt"
	"testing"
	"time"
)

func TestCredentialBudgetBoundsDistributedHashingAndResets(t *testing.T) {
	a := &authState{attempts: map[string]attemptWindow{}}
	now := time.Now()
	for n := 0; n < 60; n++ {
		if !a.allowCredentialAttempt(fmt.Sprint("source-", n/10), now) {
			t.Fatal("budget rejected before its bound", n)
		}
	}
	if a.allowCredentialAttempt("another-source", now) {
		t.Fatal("distributed attempts bypassed owner budget")
	}
	if !a.allowCredentialAttempt("another-source", now.Add(time.Minute+time.Second)) {
		t.Fatal("expired budget permanently locked the owner")
	}
}

func TestRejectedSourceDoesNotConsumeOtherSourceCredentialBudget(t *testing.T) {
	a := &authState{attempts: map[string]attemptWindow{}}
	now := time.Now()
	for i := 0; i < 10; i++ {
		if !a.allowCredentialAttempt("one", now) {
			t.Fatal(i)
		}
	}
	for i := 0; i < 1000; i++ {
		if a.allowCredentialAttempt("one", now) {
			t.Fatal("source budget ignored")
		}
	}
	if !a.allowCredentialAttempt("two", now) {
		t.Fatal("blocked source consumed global budget")
	}
}
