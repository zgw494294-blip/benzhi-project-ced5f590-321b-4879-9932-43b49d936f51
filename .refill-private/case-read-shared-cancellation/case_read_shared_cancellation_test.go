package case_read_shared_cancellation_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

type observedContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (c *observedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

func TestActiveReaderSurvivesCanceledCoalescedLeader(t *testing.T) {
	repository, err := store.Open("file:case-read-shared-cancellation?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	service := application.New(repository).WithClock(func() time.Time { return now })
	_, err = service.CreateCase(context.Background(), application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{Actor: "巡护员", Role: domain.RolePatrol, IdempotencyKey: "create"},
		ID:          "case-context", TreeCode: "GS-CONTEXT", Location: "东侧树坛", ProtectionGrade: "一级", Owner: "巡护组",
		WorkWindowStart: now.Add(time.Hour), WorkWindowEnd: now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	reserved, err := repository.DB().Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	leaderBase, cancelLeader := context.WithCancel(context.Background())
	leaderObserved := make(chan struct{})
	leaderContext := &observedContext{Context: leaderBase, observed: leaderObserved}
	leaderResult := make(chan error, 1)
	go func() {
		_, err := service.Get(leaderContext, "case-context")
		leaderResult <- err
	}()
	<-leaderObserved

	followerObserved := make(chan struct{})
	followerContext := &observedContext{Context: context.Background(), observed: followerObserved}
	followerResult := make(chan error, 1)
	go func() {
		_, err := service.Get(followerContext, "case-context")
		followerResult <- err
	}()
	<-followerObserved

	cancelLeader()
	if err := <-leaderResult; err == nil {
		t.Fatal("被取消的首个读取请求应返回 context 错误")
	}
	if err := reserved.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-followerResult; err != nil {
		t.Fatalf("仍然有效的并发读取请求不应继承其他请求的取消状态: %v", err)
	}
}
