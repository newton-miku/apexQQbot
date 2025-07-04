package apexapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type PlayerBindingData struct {
	QQ             string
	EAID           string
	EAUID          string
	LastUpdateTime time.Time
	LastRankScore  int
}

type PlayerData struct {
	data map[string]PlayerBindingData
	Lock sync.RWMutex
}

const bindingFile = "conf/eaid_bindings.json"

var Players = PlayerData{
	data: make(map[string]PlayerBindingData),
}

func (p *PlayerData) GetData() map[string]PlayerBindingData {
	p.Lock.RLock()
	defer p.Lock.RUnlock()
	return p.data
}

func (p *PlayerData) Get(qqID string) (PlayerBindingData, bool) {
	p.Lock.RLock()
	defer p.Lock.RUnlock()
	binding, ok := p.data[qqID]
	return binding, ok
}
func (p *PlayerData) GetUIDbyQQ(qqID string) (string, bool) {
	p.Lock.RLock()
	defer p.Lock.RUnlock()
	binding, ok := p.data[qqID]
	return binding.EAUID, ok
}
func (p *PlayerData) GetRankscoreByQQ(qqID string) int {
	p.Lock.RLock()
	defer p.Lock.RUnlock()
	binding, ok := p.data[qqID]
	if ok {
		return binding.LastRankScore
	}
	return 0
}
func (p *PlayerData) GetEAIDbyQQ(qqID string) (string, bool) {
	p.Lock.RLock()
	defer p.Lock.RUnlock()
	binding, ok := p.data[qqID]
	return binding.EAID, ok
}

// 导出 Set 方法用于设置绑定数据
func (p *PlayerData) Set(qqID string, binding PlayerBindingData) {
	p.Lock.Lock()
	defer p.SaveBindingRecords()
	p.data[qqID] = binding
	defer p.Lock.Unlock()
}

func (p *PlayerData) SaveBindingRecords() error {

	// 清理路径，防止路径穿越攻击
	cleanedPath := filepath.Clean(bindingFile)

	// 创建临时文件写入，避免原文件被破坏
	tempFile, err := os.CreateTemp(filepath.Dir(cleanedPath), "binding_*.json")
	if err != nil {
		return err
	}
	defer func() {
		// tempFile.Close()
		os.Remove(tempFile.Name()) // 删除临时文件
	}()

	// 设置文件权限为 0644
	if err := tempFile.Chmod(0644); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	Players.Lock.RLock()
	defer Players.Lock.RUnlock()
	encoder := json.NewEncoder(tempFile)
	if err := encoder.Encode(Players.data); err != nil {
		return err
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	// 原子替换文件
	return os.Rename(tempFile.Name(), cleanedPath)
}

func (p *PlayerData) loadPlayerData() error {
	bindings, err := p.loadBindingRecords()
	if err != nil {
		return err
	}
	if bindings == nil {
		bindings = make(map[string]PlayerBindingData)
	}
	p.Lock.Lock()
	defer Players.Lock.Unlock()
	p.data = bindings

	return nil
}

func (p *PlayerData) loadBindingRecords() (map[string]PlayerBindingData, error) {
	file, err := os.Open(bindingFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var bindings map[string]PlayerBindingData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&bindings); err != nil {
		return nil, err
	}

	return bindings, nil
}
