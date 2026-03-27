package apexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	dbFile        = "conf/apexbot.db"
	bindingFile   = "conf/eaid_bindings.json"
	migrationSQL = `
		CREATE TABLE IF NOT EXISTS player_bindings (
			qq_id TEXT PRIMARY KEY,
			ea_id TEXT NOT NULL,
			ea_uid TEXT NOT NULL,
			last_update_time INTEGER NOT NULL,
			last_rank_score INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_eaid ON player_bindings(ea_id);
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA busy_timeout=5000;
		PRAGMA cache_size=-2000;
	`
)

type PlayerBindingData struct {
	QQ             string    `json:"qq_id"`
	EAID           string    `json:"ea_id"`
	EAUID          string    `json:"ea_uid,omitempty"`
	LastUpdateTime time.Time `json:"LastUpdateTime"`
	LastRankScore  int       `json:"LastRankScore"`
}

type PlayerData struct {
	db       *sql.DB
	Lock     sync.RWMutex
	initOnce sync.Once
	initErr  error
}

var Players = PlayerData{
	db: nil,
}

// 初始化 SQLite 数据库
func (p *PlayerData) Init() error {
	p.initOnce.Do(func() {
		// 确保目录存在
		dir := filepath.Dir(dbFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			p.initErr = fmt.Errorf("创建数据库目录失败: %w", err)
			return
		}

		// 连接数据库
		db, err := sql.Open("sqlite", dbFile)
		if err != nil {
			p.initErr = fmt.Errorf("打开数据库失败: %w", err)
			return
		}

		// 设置连接池
		// WAL 模式：读可以并发，写串行化
		db.SetMaxOpenConns(5) // 允许 5 个并发连接（读多写少场景）
		db.SetMaxIdleConns(5)

		// 执行迁移
		if _, err := db.Exec(migrationSQL); err != nil {
			p.initErr = fmt.Errorf("执行数据库迁移失败: %w", err)
			db.Close()
			return
		}

		p.db = db
	})

	return p.initErr
}

// 关闭数据库连接
func (p *PlayerData) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// 检查数据库是否已初始化
func (p *PlayerData) ensureInit() error {
	if p.db == nil {
		return p.Init()
	}
	return nil
}

// Get 获取绑定数据
func (p *PlayerData) Get(qqID string) (PlayerBindingData, bool) {
	p.Lock.RLock()
	defer p.Lock.RUnlock()

	if err := p.ensureInit(); err != nil {
		return PlayerBindingData{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var binding PlayerBindingData
	var timestamp int64
	err := p.db.QueryRowContext(ctx, `
		SELECT qq_id, ea_id, ea_uid, last_update_time, last_rank_score
		FROM player_bindings WHERE qq_id = ?
	`, qqID).Scan(
		&binding.QQ,
		&binding.EAID,
		&binding.EAUID,
		&timestamp,
		&binding.LastRankScore,
	)

	if err == sql.ErrNoRows {
		return PlayerBindingData{}, false
	}
	if err != nil {
		return PlayerBindingData{}, false
	}

	binding.LastUpdateTime = time.Unix(timestamp, 0)
	return binding, true
}

// GetUIDbyQQ 通过 QQ ID 获取 EA UID
func (p *PlayerData) GetUIDbyQQ(qqID string) (string, bool) {
	binding, ok := p.Get(qqID)
	return binding.EAUID, ok
}

// GetRankscoreByQQ 通过 QQ ID 获取段位分数
func (p *PlayerData) GetRankscoreByQQ(qqID string) int {
	binding, ok := p.Get(qqID)
	if !ok {
		return 0
	}
	return binding.LastRankScore
}

// GetEAIDbyQQ 通过 QQ ID 获取 EA ID
func (p *PlayerData) GetEAIDbyQQ(qqID string) (string, bool) {
	binding, ok := p.Get(qqID)
	return binding.EAID, ok
}

// Set 设置绑定数据
func (p *PlayerData) Set(qqID string, binding PlayerBindingData) {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	if err := p.ensureInit(); err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用 REPLACE INTO 实现 upsert
	_, err := p.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO player_bindings (qq_id, ea_id, ea_uid, last_update_time, last_rank_score)
		VALUES (?, ?, ?, ?, ?)
	`, qqID, binding.EAID, binding.EAUID, binding.LastUpdateTime.Unix(), binding.LastRankScore)

	if err != nil {
		// 记录错误但不阻塞
		fmt.Printf("保存绑定数据失败: %v\n", err)
	}
}

// Delete 删除绑定
func (p *PlayerData) Delete(qqID string) error {
	p.Lock.Lock()
	defer p.Lock.Unlock()

	if err := p.ensureInit(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := p.db.ExecContext(ctx, "DELETE FROM player_bindings WHERE qq_id = ?", qqID)
	return err
}

// GetAll 获取所有绑定数据
func (p *PlayerData) GetAll() ([]PlayerBindingData, error) {
	p.Lock.RLock()
	defer p.Lock.RUnlock()

	if err := p.ensureInit(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := p.db.QueryContext(ctx, `
		SELECT qq_id, ea_id, ea_uid, last_update_time, last_rank_score
		FROM player_bindings
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bindings []PlayerBindingData
	for rows.Next() {
		var binding PlayerBindingData
		var timestamp int64
		if err := rows.Scan(
			&binding.QQ,
			&binding.EAID,
			&binding.EAUID,
			&timestamp,
			&binding.LastRankScore,
		); err != nil {
			return nil, err
		}
		binding.LastUpdateTime = time.Unix(timestamp, 0)
		bindings = append(bindings, binding)
	}

	return bindings, rows.Err()
}

// GetData 获取 map 格式的绑定数据（兼容性方法）
func (p *PlayerData) GetData() map[string]PlayerBindingData {
	bindings, err := p.GetAll()
	if err != nil {
		return make(map[string]PlayerBindingData)
	}

	result := make(map[string]PlayerBindingData)
	for _, b := range bindings {
		result[b.QQ] = b
	}
	return result
}

// 迁移旧的 JSON 数据到 SQLite
func (p *PlayerData) AutoMigrate() error {
	// 检查 JSON 文件是否存在
	if _, err := os.Stat(bindingFile); os.IsNotExist(err) {
		return nil // 没有旧文件，无需迁移
	}

	// 加载旧数据
	oldData, err := p.loadBindingRecords()
	if err != nil {
		return fmt.Errorf("加载旧数据失败: %w", err)
	}
	if oldData == nil {
		return nil
	}

	// 迁移到 SQLite
	p.Lock.Lock()
	defer p.Lock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO player_bindings (qq_id, ea_id, ea_uid, last_update_time, last_rank_score)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	migratedCount := 0
	for qqID, binding := range oldData {
		// 解析时间
		var updateTime time.Time
		if !binding.LastUpdateTime.IsZero() {
			updateTime = binding.LastUpdateTime
		} else {
			updateTime = time.Now()
		}

		_, err := stmt.ExecContext(ctx,
			qqID,
			binding.EAID,
			binding.EAUID,
			updateTime.Unix(),
			binding.LastRankScore,
		)
		if err != nil {
			continue
		}
		migratedCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	// 迁移成功后备份旧文件
	bakFile := bindingFile + ".bak"
	if err := os.Rename(bindingFile, bakFile); err != nil {
		// 静默失败
	} else {
		// 静默成功
	}

	return nil
}

// MigrateFromJSON 兼容旧版本（已废弃，使用 AutoMigrate）
func (p *PlayerData) MigrateFromJSON() error {
	return p.AutoMigrate()
}

// loadBindingRecords 加载旧的 JSON 数据（仅用于迁移）
func (p *PlayerData) loadBindingRecords() (map[string]PlayerBindingData, error) {
	file, err := os.Open(bindingFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 使用原始字节解码，检测时间格式
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// 先尝试解析为旧格式（时间是字符串）
	var oldFormat struct {
		Data map[string]struct {
			QQ             string `json:"qq"`
			EAID           string `json:"ea_id"`
			LastUpdateTime string `json:"LastUpdateTime"`
			LastRankScore  int    `json:"LastRankScore"`
		} `json:"-"`
	}

	if err := json.Unmarshal(data, &oldFormat); err != nil {
		// 如果旧格式解析失败，尝试新格式
		var bindings map[string]PlayerBindingData
		if err := json.Unmarshal(data, &bindings); err != nil {
			return nil, err
		}
		return bindings, nil
	}

	// 转换为新格式
	bindings := make(map[string]PlayerBindingData)
	for qqID, item := range oldFormat.Data {
		binding := PlayerBindingData{
			QQ:            qqID,
			EAID:          item.EAID,
			EAUID:         "",
			LastRankScore: item.LastRankScore,
		}

		// 解析时间字符串
		if item.LastUpdateTime != "" {
			t, err := parseOldTime(item.LastUpdateTime)
			if err == nil {
				binding.LastUpdateTime = t
			}
		}
		bindings[qqID] = binding
	}

	return bindings, nil
}

// parseOldTime 解析旧时间格式
func parseOldTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.9999999+08:00",
		"2006-01-02T15:04:05+08:00",
		"2006-01-02T15:04:05",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析时间: %s", s)
}

// SaveBindingRecords 保持兼容性（空实现，数据已实时写入数据库）
func (p *PlayerData) SaveBindingRecords() error {
	return nil
}

// loadPlayerData 加载数据（空实现，已改为惰性加载）
func (p *PlayerData) loadPlayerData() error {
	return nil
}
