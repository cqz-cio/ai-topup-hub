package common

import (
	"encoding/json"
	"fmt"
)

// ConfigNormalizer 配置归一化约束，要求指针接收器实现 Normalize()。
type ConfigNormalizer[T any] interface {
	*T
	Normalize()
}

// ParseConfig 通用配置解析：JSON marshal/unmarshal + Normalize。
func ParseConfig[T any, PT ConfigNormalizer[T]](raw map[string]interface{}, errConfigInvalid error) (*T, error) {
	if raw == nil {
		return nil, fmt.Errorf("%w: empty config", errConfigInvalid)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal config failed", errConfigInvalid)
	}
	var cfg T
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: unmarshal config failed", errConfigInvalid)
	}
	PT(&cfg).Normalize()
	return &cfg, nil
}
