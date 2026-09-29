package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestBackupInfoRoundTrip 元数据要能原样写回读出——回滚提示全靠它，
// 丢字段等于把「回滚到哪个版本、安不安全」这些信息一起丢掉。
func TestBackupInfoRoundTrip(t *testing.T) {
	exec := filepath.Join(t.TempDir(), "dujiao-next")
	started := time.Now().Truncate(time.Second)
	want := BackupInfo{
		PreviousVersion:     "v1.4.0",
		TargetVersion:       "v1.5.0",
		SwappedAt:           started,
		MigrationStarted:    true,
		MigrationStartedAt:  &started,
		NewVersionStarted:   true,
		NewVersionStartedAt: &started,
	}

	if err := writeBackupInfo(exec, want); err != nil {
		t.Fatalf("writeBackupInfo: %v", err)
	}
	got, known, err := ReadBackupInfo(exec)
	if err != nil {
		t.Fatalf("ReadBackupInfo: %v", err)
	}
	if !known {
		t.Fatal("a complete metadata file must be reported as known")
	}
	if got.PreviousVersion != want.PreviousVersion || got.TargetVersion != want.TargetVersion {
		t.Errorf("versions = %q -> %q, want %q -> %q",
			got.PreviousVersion, got.TargetVersion, want.PreviousVersion, want.TargetVersion)
	}
	if !got.NewVersionStarted {
		t.Error("NewVersionStarted lost in round trip")
	}
	if !got.MigrationStarted || got.MigrationStartedAt == nil {
		t.Error("MigrationStarted state lost in round trip")
	}
	if !got.SwappedAt.Equal(want.SwappedAt) {
		t.Errorf("SwappedAt = %v, want %v", got.SwappedAt, want.SwappedAt)
	}
}

// TestReadBackupInfoMissingIsNotAnError 从旧版本升上来的那一次没有元数据文件，
// 这属于正常情况，不能报错——否则回滚会被一个「信息缺失」挡住。
func TestReadBackupInfoMissingIsNotAnError(t *testing.T) {
	exec := filepath.Join(t.TempDir(), "dujiao-next")
	info, known, err := ReadBackupInfo(exec)
	if err != nil {
		t.Fatalf("missing metadata should not error, got %v", err)
	}
	if known {
		t.Error("missing metadata must be reported as unknown, not as a valid zero value")
	}
	if info.TargetVersion != "" {
		t.Errorf("expected zero value, got %+v", info)
	}
}

// TestReadBackupInfoCorruptedIsNotAnError 元数据损坏时不返回错误，但必须报 known=false：
// 读不动的文件证明不了新版本没跑过，调用方要按 unsafe 处理。
func TestReadBackupInfoCorruptedIsNotAnError(t *testing.T) {
	exec := filepath.Join(t.TempDir(), "dujiao-next")
	if err := os.WriteFile(metadataPath(exec), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, known, err := ReadBackupInfo(exec)
	if err != nil {
		t.Fatalf("corrupted metadata should not error, got %v", err)
	}
	if known {
		t.Error("corrupted metadata must be reported as unknown")
	}
	if info.TargetVersion != "" {
		t.Errorf("expected zero value, got %+v", info)
	}
}

// seedUpgraded 造出一个「刚升级完」的现场：exec 是新二进制，.backup 是旧二进制。
func seedUpgraded(t *testing.T, info BackupInfo) string {
	t.Helper()
	exec := filepath.Join(t.TempDir(), "dujiao-next")
	if err := os.WriteFile(exec, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath(exec), []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeBackupInfo(exec, info); err != nil {
		t.Fatal(err)
	}
	return exec
}

// TestRollbackRefusesAfterNewVersionStarted 新版本跑通后 AutoMigrate 已经推进过 schema，
// 默认必须拒绝回滚，避免用户点一个看起来无害的按钮就把库和二进制搞成不匹配状态。
func TestRollbackRefusesAfterNewVersionStarted(t *testing.T) {
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion:   "v1.4.0",
		TargetVersion:     "v1.5.0",
		NewVersionStarted: true,
	})

	if err := rollbackAt(exec, false); err != ErrRollbackUnsafe {
		t.Fatalf("rollbackAt(force=false) = %v, want ErrRollbackUnsafe", err)
	}
	// 被拒绝时不能动任何文件
	if data, _ := os.ReadFile(exec); string(data) != "new-binary" {
		t.Errorf("binary must stay untouched when rollback is refused, got %q", data)
	}
	if _, err := os.Stat(backupPath(exec)); err != nil {
		t.Error("backup must survive a refused rollback")
	}
}

// TestRollbackRefusesAfterMigrationStarted 即使迁移最终失败、服务没有完整启动，
// schema 也可能已经部分推进，普通回滚必须被拒绝。
func TestRollbackRefusesAfterMigrationStarted(t *testing.T) {
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion:  "v1.4.0",
		TargetVersion:    "v1.5.0",
		MigrationStarted: true,
	})

	if err := rollbackAt(exec, false); !errors.Is(err, ErrRollbackUnsafe) {
		t.Fatalf("rollbackAt(force=false) = %v, want ErrRollbackUnsafe", err)
	}
	if data, _ := os.ReadFile(exec); string(data) != "new-binary" {
		t.Errorf("binary must stay untouched when migration has started, got %q", data)
	}
}

// TestRollbackForceAfterNewVersionStarted 显式 force 时放行，并消耗掉备份与元数据。
func TestRollbackForceAfterNewVersionStarted(t *testing.T) {
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion:   "v1.4.0",
		TargetVersion:     "v1.5.0",
		NewVersionStarted: true,
	})

	if err := rollbackAt(exec, true); err != nil {
		t.Fatalf("rollbackAt(force=true) = %v, want nil", err)
	}
	if data, _ := os.ReadFile(exec); string(data) != "old-binary" {
		t.Errorf("binary after rollback = %q, want old-binary", data)
	}
	if _, err := os.Stat(backupPath(exec)); !os.IsNotExist(err) {
		t.Error("backup should be consumed by rollback")
	}
	if _, err := os.Stat(metadataPath(exec)); !os.IsNotExist(err) {
		t.Error("metadata should be removed after rollback")
	}
	if _, err := os.Stat(rollbackMarkerPath(exec)); err != nil {
		t.Errorf("rollback marker should remain for an already loaded new process: %v", err)
	}
}

// TestRollbackAllowedBeforeNewVersionStarted 二进制已替换但新版本还没起来过，
// 这才是回滚真正安全的窗口，不该要求 force。
func TestRollbackAllowedBeforeNewVersionStarted(t *testing.T) {
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion: "v1.4.0",
		TargetVersion:   "v1.5.0",
	})

	if err := rollbackAt(exec, false); err != nil {
		t.Fatalf("rollbackAt = %v, want nil", err)
	}
	if data, _ := os.ReadFile(exec); string(data) != "old-binary" {
		t.Errorf("binary after rollback = %q, want old-binary", data)
	}
}

// TestStartupGuardMarksMigrationBeforeCompletion 启动闸门先记录迁移开始，
// 完整启动状态只能在迁移与初始化完成后补上。
func TestStartupGuardMarksMigrationBeforeCompletion(t *testing.T) {
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion: "v1.4.0",
		TargetVersion:   "v1.5.0",
	})

	guard, err := acquireStartupGuardAt(exec, "v1.5.0")
	if err != nil {
		t.Fatalf("acquireStartupGuardAt: %v", err)
	}
	defer guard.Release()

	if err := guard.MarkMigrationStarted(); err != nil {
		t.Fatalf("MarkMigrationStarted: %v", err)
	}
	migrating, _, err := ReadBackupInfo(exec)
	if err != nil {
		t.Fatal(err)
	}
	if !migrating.MigrationStarted {
		t.Error("migration start must be persisted before AutoMigrate")
	}
	if migrating.NewVersionStarted {
		t.Error("migration start must not claim the service fully started")
	}

	if err := guard.MarkStartupCompleted(); err != nil {
		t.Fatalf("MarkStartupCompleted: %v", err)
	}
	info, _, err := ReadBackupInfo(exec)
	if err != nil {
		t.Fatal(err)
	}
	if !info.NewVersionStarted {
		t.Error("matching version should mark NewVersionStarted")
	}
	if info.NewVersionStartedAt == nil {
		t.Error("NewVersionStartedAt should be recorded")
	}
}

// TestStartupGuardTreatsForeignMetadataAsUnknown 运行版本与升级元数据目标不一致时，
// 必须按「这份元数据不描述当前二进制」处理：启动照常进行，元数据被保守记录覆盖，
// 回滚仍然 fail-closed。绝不能拒绝启动 —— 见下面的手工升级用例。
func TestStartupGuardTreatsForeignMetadataAsUnknown(t *testing.T) {
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion: "v1.4.0",
		TargetVersion:   "v1.5.0",
	})

	guard, err := acquireStartupGuardAt(exec, "v1.4.0")
	if err != nil {
		t.Fatalf("acquireStartupGuardAt mismatch = %v, want nil", err)
	}
	if err := guard.MarkMigrationStarted(); err != nil {
		t.Fatalf("MarkMigrationStarted: %v", err)
	}
	guard.Release()

	info, known, err := ReadBackupInfo(exec)
	if err != nil {
		t.Fatal(err)
	}
	if !known {
		t.Fatal("startup should have rewritten metadata describing the running binary")
	}
	if info.TargetVersion != "v1.4.0" {
		t.Errorf("TargetVersion = %q, want the running version v1.4.0", info.TargetVersion)
	}
	if info.PreviousVersion != "" {
		t.Errorf("PreviousVersion = %q, want empty: a foreign record cannot vouch for .backup", info.PreviousVersion)
	}
	if !info.MigrationStarted {
		t.Error("MigrationStarted must be recorded before AutoMigrate")
	}
	if err := rollbackAt(exec, false); err != ErrRollbackUnsafe {
		t.Fatalf("rollback = %v, want ErrRollbackUnsafe", err)
	}
}

// TestManualUpgradeAfterSelfUpdateStillBoots 复现文档「9. 升级」记载的手工升级流程：
//
//	后台一键升级 v1.3.1 → v1.4.0（留下 .backup 与 .backup.json{target: v1.4.0}）
//	→ 之后按文档下载 tar.gz 手工把二进制换成 v1.5.0（两个文件原样留在目录里）
//	→ systemctl restart
//
// .backup 只有下一次 swap 或 rollback 才会被清理，也就是说它会无限期存在。
// 一旦把「元数据目标对不上」当成「已被回滚取代」而退出，这条文档主路径会让服务
// 每次启动都失败，且日志只会说自己被回滚了 —— 现场没有任何回滚发生过。
func TestManualUpgradeAfterSelfUpdateStillBoots(t *testing.T) {
	exec := filepath.Join(t.TempDir(), "dujiao-next")
	if err := os.WriteFile(exec, []byte("v1.4.0-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath(exec), []byte("v1.3.1-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 一键升级到 v1.4.0 并完整启动过一次后的稳定状态
	guard, err := acquireStartupGuardAt(exec, "v1.4.0")
	if err != nil {
		t.Fatalf("v1.4.0 first startup: %v", err)
	}
	if err := guard.MarkStartupCompleted(); err != nil {
		t.Fatal(err)
	}
	guard.Release()

	// 用户按文档只替换二进制，.backup / .backup.json 都还在
	if err := os.WriteFile(exec, []byte("v1.5.0-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 连续两次重启都必须能正常起来，确认不是「退出一次就好」
	for i, attempt := range []string{"first", "second"} {
		g, err := acquireStartupGuardAt(exec, "v1.5.0")
		if err != nil {
			t.Fatalf("%s startup after manual upgrade = %v, want nil (main.go exits 1 on any error)", attempt, err)
		}
		if err := g.MarkMigrationStarted(); err != nil {
			t.Fatalf("%s MarkMigrationStarted: %v", attempt, err)
		}
		g.Release()
		if i == 0 {
			// 第一次启动后元数据应该已经描述 v1.5.0
			info, known, err := ReadBackupInfo(exec)
			if err != nil || !known {
				t.Fatalf("ReadBackupInfo = (%+v, %v, %v)", info, known, err)
			}
			if info.TargetVersion != "v1.5.0" {
				t.Errorf("TargetVersion = %q, want v1.5.0", info.TargetVersion)
			}
		}
	}

	// 手工升级过的目录里，回滚目标已经无从证明，必须仍然要求 force
	if err := rollbackAt(exec, false); err != ErrRollbackUnsafe {
		t.Fatalf("rollback after manual upgrade = %v, want ErrRollbackUnsafe", err)
	}
}

// makeInstallDirReadOnly 把测试目录切成服务账号不可写，并在用例结束时恢复，
// 以免 TempDir 清理失败。root 会绕过权限位，调用方需要先 skip。
func makeInstallDirReadOnly(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// TestStartupGuardReadOnlyStateFailsClosedBeforeMigration 安装目录不可写时，如果
// 仍存在一个尚未记录迁移的 backup 状态，就不能把 MarkMigrationStarted 降级成
// 成功的空操作。否则 main 会继续 AutoMigrate，旧元数据却仍会放行普通回滚。
func TestStartupGuardReadOnlyStateFailsClosedBeforeMigration(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 会无视目录权限位，无法在此模拟只读安装目录")
	}
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion: "v1.4.0",
		TargetVersion:   "v1.5.0",
	})
	// 一键升级流程在替换前已经创建过锁文件。目录后来变成只读时，仍应能打开
	// 现有锁并进入 guard；真正不能持久化 migration_started 时再 fail-closed。
	if err := os.WriteFile(exec+".lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	makeInstallDirReadOnly(t, filepath.Dir(exec))

	guard, err := acquireStartupGuardAt(exec, "v1.5.0")
	if err != nil {
		t.Fatalf("acquireStartupGuardAt with existing lock = %v, want nil", err)
	}
	defer guard.Release()

	if err := guard.MarkMigrationStarted(); err == nil {
		t.Fatal("MarkMigrationStarted on read-only update state = nil, want fail-closed error before AutoMigrate")
	}

	info, known, err := ReadBackupInfo(exec)
	if err != nil || !known {
		t.Fatalf("ReadBackupInfo = (%+v, %v, %v)", info, known, err)
	}
	if info.MigrationStarted {
		t.Error("failed persistence must not pretend migration_started was committed")
	}
}

// TestStartupGuardHonorsRollbackMarkerInReadOnlyInstallDir 只读目录也不能绕过
// 已完成回滚留下的 marker。无法消费 marker 最多导致持续 fail-closed，不能让
// 已被取代、但仍加载在内存里的新进程继续连接数据库。
func TestStartupGuardHonorsRollbackMarkerInReadOnlyInstallDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 会无视目录权限位，无法在此模拟只读安装目录")
	}
	dir := t.TempDir()
	exec := filepath.Join(dir, "dujiao-next")
	if err := os.WriteFile(exec+".lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeRollbackMarker(exec, "v1.5.0"); err != nil {
		t.Fatal(err)
	}
	makeInstallDirReadOnly(t, dir)

	if _, err := acquireStartupGuardAt(exec, "v1.5.0"); !errors.Is(err, ErrStartupSuperseded) {
		t.Fatalf("startup with matching rollback marker = %v, want ErrStartupSuperseded", err)
	}
}

// TestStartupGuardAllowsReadOnlyInstallAfterUnsafeStatePersisted 已经可靠记录过迁移和
// 完整启动时，后续只读启动不需要再次写元数据。只要升级流程留下的锁文件仍可打开，
// guard 可以继续保护状态而不制造无意义的启动阻断。
func TestStartupGuardAllowsReadOnlyInstallAfterUnsafeStatePersisted(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 会无视目录权限位，无法在此模拟只读安装目录")
	}
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion:   "v1.4.0",
		TargetVersion:     "v1.5.0",
		MigrationStarted:  true,
		NewVersionStarted: true,
	})
	if err := os.WriteFile(exec+".lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	makeInstallDirReadOnly(t, filepath.Dir(exec))

	guard, err := acquireStartupGuardAt(exec, "v1.5.0")
	if err != nil {
		t.Fatalf("acquireStartupGuardAt stable read-only state: %v", err)
	}
	defer guard.Release()
	if err := guard.MarkMigrationStarted(); err != nil {
		t.Fatalf("already-persisted migration state should not need a write: %v", err)
	}
	if err := guard.MarkStartupCompleted(); err != nil {
		t.Fatalf("already-persisted startup state should not need a write: %v", err)
	}
}

// TestFirstUpgradeFromLegacyVersionIsUnsafe 复现真实的跨版本升级时序：
//
//	旧版本进程执行替换 → 只留下 .backup，没有元数据（旧代码里根本没这段逻辑）
//	→ 新二进制启动、跑完 AutoMigrate
//	→ 此时回滚必须被拦下
//
// 这是回滚闸门第一次上线时**所有存量实例**都会走的路径。如果这里默认放行，
// 那这道闸门对它最该保护的那批用户完全不起作用。
func TestFirstUpgradeFromLegacyVersionIsUnsafe(t *testing.T) {
	exec := filepath.Join(t.TempDir(), "dujiao-next")
	if err := os.WriteFile(exec, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 旧版本只会留下这一个文件
	if err := os.WriteFile(backupPath(exec), []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 元数据缺失时，回滚就应该已经是 unsafe
	if err := rollbackAt(exec, false); err != ErrRollbackUnsafe {
		t.Fatalf("rollback with missing metadata = %v, want ErrRollbackUnsafe", err)
	}

	// 新版本启动，在 AutoMigrate 前回填保守元数据并持有跨进程锁
	guard, err := acquireStartupGuardAt(exec, "v1.5.0")
	if err != nil {
		t.Fatalf("acquireStartupGuardAt: %v", err)
	}
	if err := guard.MarkMigrationStarted(); err != nil {
		t.Fatalf("MarkMigrationStarted: %v", err)
	}
	if err := guard.MarkStartupCompleted(); err != nil {
		t.Fatalf("MarkStartupCompleted: %v", err)
	}
	guard.Release()

	info, known, err := ReadBackupInfo(exec)
	if err != nil {
		t.Fatal(err)
	}
	if !known {
		t.Fatal("startup should have backfilled usable metadata")
	}
	if !info.NewVersionStarted {
		t.Error("backfilled metadata must record that the new version has started")
	}
	if !info.MigrationStarted {
		t.Error("backfilled metadata must record that database migration started")
	}
	if info.PreviousVersion != "" {
		t.Errorf("PreviousVersion = %q, want empty (the old binary's version is genuinely unknown)", info.PreviousVersion)
	}

	// 回填之后依然必须要求 force
	if err := rollbackAt(exec, false); err != ErrRollbackUnsafe {
		t.Fatalf("rollback after backfill = %v, want ErrRollbackUnsafe", err)
	}
	if err := rollbackAt(exec, true); err != nil {
		t.Fatalf("forced rollback = %v, want nil", err)
	}
	if data, _ := os.ReadFile(exec); string(data) != "old-binary" {
		t.Errorf("binary after forced rollback = %q, want old-binary", data)
	}

	// 已经加载进内存的 v1.5.0 进程必须看到 marker 并在迁移前退出。
	if _, err := acquireStartupGuardAt(exec, "v1.5.0"); !errors.Is(err, ErrStartupSuperseded) {
		t.Fatalf("superseded process startup = %v, want ErrStartupSuperseded", err)
	}
	if _, err := os.Stat(rollbackMarkerPath(exec)); !os.IsNotExist(err) {
		t.Error("superseded process should consume the rollback marker before exiting")
	}
	// 真正从磁盘启动的旧版本随后可以继续。
	oldGuard, err := acquireStartupGuardAt(exec, "v1.4.0")
	if err != nil {
		t.Fatalf("rolled-back version startup: %v", err)
	}
	oldGuard.Release()
}

// TestConsumedRollbackMarkerDoesNotPermanentlyBlockSameVersion 历史版本没有
// StartupGuard，回滚标记可能一直留到下一次重新升级。它可以让首次启动安全退出一次，
// 但必须被消费；否则同一版本之后每次重启都会被永久挡住。
func TestConsumedRollbackMarkerDoesNotPermanentlyBlockSameVersion(t *testing.T) {
	exec := filepath.Join(t.TempDir(), "dujiao-next")
	if err := writeRollbackMarker(exec, "v1.5.0"); err != nil {
		t.Fatal(err)
	}

	if _, err := acquireStartupGuardAt(exec, "v1.5.0"); !errors.Is(err, ErrStartupSuperseded) {
		t.Fatalf("first startup = %v, want ErrStartupSuperseded", err)
	}
	guard, err := acquireStartupGuardAt(exec, "v1.5.0")
	if err != nil {
		t.Fatalf("second startup should proceed after marker consumption: %v", err)
	}
	guard.Release()
}

// TestRollbackRefusesOnCorruptedMetadata 元数据损坏同样无法证明安全，默认必须拒绝。
func TestRollbackRefusesOnCorruptedMetadata(t *testing.T) {
	exec := filepath.Join(t.TempDir(), "dujiao-next")
	if err := os.WriteFile(exec, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath(exec), []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath(exec), []byte("{truncated"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := rollbackAt(exec, false); err != ErrRollbackUnsafe {
		t.Fatalf("rollback with corrupted metadata = %v, want ErrRollbackUnsafe", err)
	}
}

// TestMigrationFailureStateRemainsUnsafe 模拟迁移开始后进程直接失败：
// 即使没有 MarkStartupCompleted，普通回滚仍必须被挡住。
func TestMigrationFailureStateRemainsUnsafe(t *testing.T) {
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion: "v1.4.0",
		TargetVersion:   "v1.5.0",
	})
	guard, err := acquireStartupGuardAt(exec, "v1.5.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.MarkMigrationStarted(); err != nil {
		t.Fatal(err)
	}
	// 模拟 AutoMigrate 部分写入后报错、进程退出：只释放内核锁，不写完成状态。
	guard.Release()

	if err := rollbackAt(exec, false); !errors.Is(err, ErrRollbackUnsafe) {
		t.Fatalf("rollback after failed migration = %v, want ErrRollbackUnsafe", err)
	}
}

// TestStartupGuardBlocksConcurrentRollback 迁移期间 CLI 回滚只能立即失败，不能等待后交错执行。
func TestStartupGuardBlocksConcurrentRollback(t *testing.T) {
	exec := seedUpgraded(t, BackupInfo{
		PreviousVersion: "v1.4.0",
		TargetVersion:   "v1.5.0",
	})
	guard, err := acquireStartupGuardAt(exec, "v1.5.0")
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Release()

	if err := rollbackAt(exec, false); !errors.Is(err, ErrUpdateInProgress) {
		t.Fatalf("rollback while startup guard is held = %v, want ErrUpdateInProgress", err)
	}
}

// TestStartupGuardNoBackupIsNoop 没有备份或回滚标记就没有需要串行化的窗口：
// 不仅不能写元数据，也不能创建锁文件。否则只读安装目录中的 release 二进制
// 会因为一个本来不可用的自更新功能而连正常服务都启动不了。
func TestStartupGuardNoBackupIsNoop(t *testing.T) {
	exec := filepath.Join(t.TempDir(), "dujiao-next")
	guard, err := acquireStartupGuardAt(exec, "v1.5.0")
	if err != nil {
		t.Fatalf("acquireStartupGuardAt: %v", err)
	}
	if err := guard.MarkMigrationStarted(); err != nil {
		t.Fatalf("MarkMigrationStarted: %v", err)
	}
	guard.Release()
	if _, err := os.Stat(metadataPath(exec)); !os.IsNotExist(err) {
		t.Error("no metadata should be written when there is no backup")
	}
	if _, err := os.Stat(exec + ".lock"); !os.IsNotExist(err) {
		t.Error("normal startup without update state must not create a binary lock")
	}
}
