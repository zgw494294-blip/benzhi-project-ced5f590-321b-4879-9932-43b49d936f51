package main

import "testing"

func TestValidateAddressOnlyAllowsLoopback(t *testing.T) {
	if err := validateAddress("127.0.0.1:19081"); err != nil {
		t.Fatal(err)
	}
	if err := validateAddress("0.0.0.0:19081"); err == nil {
		t.Fatal("不得接受非回环监听地址")
	}
	if err := validateAddress("127.0.0.1:70000"); err == nil {
		t.Fatal("不得接受无效端口")
	}
}

func TestPortEnvironmentOverride(t *testing.T) {
	t.Setenv("PORT", "19123")
	address, err := configuredDefaultAddress()
	if err != nil {
		t.Fatal(err)
	}
	if address != "127.0.0.1:19123" {
		t.Fatalf("PORT 覆盖错误：%s", address)
	}
	t.Setenv("PORT", "80")
	if _, err := configuredDefaultAddress(); err == nil {
		t.Fatal("不得接受低位 PORT")
	}
}
