package case_list_cache_alias_pollution_test

import (
	"context"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

func TestCaseListCacheDoesNotLeakMutableAliases(t *testing.T) {
	repository, err := store.Open("file:case-list-cache-alias?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })

	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	service := application.New(repository).WithClock(func() time.Time { return now })
	created, err := service.CreateCase(context.Background(), application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{Actor: "巡护员", Role: domain.RolePatrol, IdempotencyKey: "create-alias-case"},
		ID:          "case-alias", TreeCode: "GS-ALIAS", Location: "测试样地", ProtectionGrade: "一级", Owner: "原责任人",
		WorkWindowStart: now.Add(time.Hour), WorkWindowEnd: now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].ID != created.ID {
		t.Fatalf("首次列表结果无效：%+v", first)
	}
	first[0].Owner = "调用方污染值"

	persisted, err := repository.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Owner != "原责任人" {
		t.Fatalf("测试前提失败：SQLite 聚合被意外修改为 %q", persisted.Owner)
	}

	second, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Owner != persisted.Owner {
		t.Fatalf("列表缓存泄漏了调用方修改：缓存 owner=%q，持久化 owner=%q", second[0].Owner, persisted.Owner)
	}
}
