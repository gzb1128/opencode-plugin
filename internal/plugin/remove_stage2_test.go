package plugin

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/opencode/plugin-cli/internal/config"
	"github.com/opencode/plugin-cli/internal/marketplace"
)

// TestInstaller_Remove 验证 Remove 路径完整地把 symlink / cache / record 都清掉。
// 这是新增测试——之前 Remove() 没有任何单元测试覆盖，isWithinDir 这种防数据丢失
// 的关键守卫也完全没被验证过。
func TestInstaller_Remove(t *testing.T) {
	t.Run("happy path clears symlinks cache and record", func(t *testing.T) {
		installer, _ := setupInstallerTest(t)
		cachePath := setupInstalledPlugin(t, installer, "test-plugin", "test-market")
		paths := installer.configMgr.GetPaths()

		// 准备：symlink 已存在
		symlinkPath := filepath.Join(paths.AgentsDir, "skills", "test-skill.md")
		if _, err := os.Lstat(symlinkPath); err != nil {
			t.Fatalf("precondition: symlink should exist, got %v", err)
		}

		if err := installer.Remove("test-plugin", "test-market"); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}

		// symlink 必须被清掉
		if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
			t.Errorf("symlink %s should be removed, got err=%v", symlinkPath, err)
		}
		// cache 目录必须被清掉
		if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
			t.Errorf("cache %s should be removed, got err=%v", cachePath, err)
		}
		// install record 必须被清掉
		if _, err := installer.configMgr.GetInstallRecord("test-plugin@test-market"); err == nil {
			t.Errorf("install record should be removed")
		}
	})

	t.Run("refuses to remove path outside cache dir", func(t *testing.T) {
		installer, _ := setupInstallerTest(t)
		paths := installer.configMgr.GetPaths()

		// 构造一个 install record，其 InstallPath 在 cacheDir 之外（模拟数据损坏 / 攻击）
		externalPath := filepath.Join(paths.BaseDir, "dangerous", "not-a-plugin")
		os.MkdirAll(externalPath, 0755)
		if err := installer.configMgr.AddInstallRecord("evil@market", &config.InstallRecord{
			Scope:       "user",
			InstallPath: externalPath,
			Version:     "1.0.0",
		}); err != nil {
			t.Fatalf("AddInstallRecord: %v", err)
		}

		err := installer.Remove("evil", "market")
		if err == nil {
			t.Fatal("expected error refusing to remove path outside cache dir, got nil")
		}
		if !strings.Contains(err.Error(), "outside cache directory") {
			t.Errorf("expected 'outside cache directory' in error, got: %v", err)
		}
		// 关键：危险路径不能被删
		if _, err := os.Stat(externalPath); err != nil {
			t.Errorf("external path was deleted — data loss! err=%v", err)
		}
	})
}

// TestInstaller_Remove_PropagatesCleanupErrors 验证当 cache 删除失败时，
// Remove 返回非 nil error（CLI 必须非零退出），但 record 仍然被移除并提示用户手工清理残留。
func TestInstaller_Remove_PropagatesCleanupErrors(t *testing.T) {
	installer, _ := setupInstallerTest(t)
	cachePath := setupInstalledPlugin(t, installer, "test-plugin", "test-market")

	// 把 cache 目录里塞一个不可写的子目录，让 RemoveAll 失败。
	// 注意：这个测试在某些 root-as-user 环境下可能无法触发失败——容忍 skip。
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission-based failure cannot be triggered")
	}
	lockedDir := filepath.Join(cachePath, "locked")
	os.MkdirAll(lockedDir, 0755)
	lockedFile := filepath.Join(lockedDir, "cant-remove")
	os.WriteFile(lockedFile, []byte("x"), 0444) // 只读文件
	// 把目录改为只读，让 RemoveAll 失败
	os.Chmod(lockedDir, 0500)
	defer os.Chmod(lockedDir, 0755) // 测试结束清理

	err := installer.Remove("test-plugin", "test-market")
	if err == nil {
		t.Fatal("expected Remove to return cleanup error, got nil")
	}

	// 关键契约：即使清理失败，record 也必须被删；残留文件需要用户按错误提示手工清理。
	if _, err := installer.configMgr.GetInstallRecord("test-plugin@test-market"); err == nil {
		t.Errorf("install record should be removed even when cleanup fails")
	}
}

// TestUpdate_Stage2Failure_LeavesBackupAndPreservesRecord 锁定 P0 修复：
// Update 在真正的 Stage 2 swap 失败时不能 (a) 把 backup 删掉，也不能 (b) 把 record 覆盖成新版本。
// 这里通过把 ~/.agents/skills 改为只读，让 RemoveSymlinks 在删除旧 symlink 时失败。
func TestUpdate_Stage2Failure_LeavesBackupAndPreservesRecord(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission-based Stage 2 failure cannot be triggered")
	}

	marketPath, pluginSrcPath, manifestPath, _ := setupUpdateTestMarket(t, "my-plugin", "1.0.0")
	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()

	src := &marketplace.LocalMarketSource{Path: marketPath}
	src.SetInstallLocation(marketPath)
	if err := installer.configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(src)); err != nil {
		t.Fatalf("AddKnownMarket error: %v", err)
	}

	if err := installer.Install("my-plugin", InstallOptions{MarketName: "test-market", Scope: "user"}); err != nil {
		t.Fatalf("Install v1 failed: %v", err)
	}

	oldRecord, _ := installer.configMgr.GetInstallRecord("my-plugin@test-market")
	oldCachePath := oldRecord.InstallPath

	// bump 到 v2 让 update 触发（cachePath 不同 → 走 backup 流程），manifest 保持合法。
	writePluginManifest(t, manifestPath, "my-plugin", "2.0.0")
	if err := os.WriteFile(filepath.Join(pluginSrcPath, "skills", "test-skill.md"), []byte("# my-plugin v2.0.0 skill"), 0644); err != nil {
		t.Fatalf("write v2 skill: %v", err)
	}

	// 触发 Stage 2 的 RemoveSymlinks 失败：删除 symlink 需要 skills 目录写权限。
	skillsDir := filepath.Join(paths.AgentsDir, "skills")
	if err := os.Chmod(skillsDir, 0500); err != nil {
		t.Fatalf("chmod skills dir: %v", err)
	}
	defer os.Chmod(skillsDir, 0755)

	updateErr := installer.Update("my-plugin", InstallOptions{MarketName: "test-market", Scope: "user"})
	if updateErr == nil {
		t.Fatal("expected Update to fail during Stage 2 RemoveSymlinks, got nil")
	}

	// backup 必须保留——不能被 defer 删掉
	backupPath := oldCachePath + ".update-backup"
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("backup %s should be preserved on Stage 2 failure for manual recovery, got err=%v", backupPath, err)
	}

	// record 必须仍然指向旧版本（不能被覆盖成新版本）
	currentRecord, err := installer.configMgr.GetInstallRecord("my-plugin@test-market")
	if err != nil {
		t.Fatalf("record lookup error: %v", err)
	}
	if currentRecord.Version != oldRecord.Version {
		t.Errorf("record version = %q, want %q (must NOT be overwritten on Stage 2 failure)",
			currentRecord.Version, oldRecord.Version)
	}
}

// TestConfig_ConcurrentWrites 锁定 atomic write + 验证并发场景下 installed_plugins.json
// 不会损坏。之前所有写都是 os.WriteFile 直接覆盖，两个 goroutine 同时写极易产生
// 截断的 JSON 让后续 CLI 调用全部失败。
func TestConfig_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	paths := config.TestEnvironment(tmpDir).Paths()
	os.MkdirAll(paths.BaseDir, 0755)
	mgr := config.NewManagerWithPath(paths)

	const goroutines = 20
	const iterations = 30
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for n := 0; n < iterations; n++ {
				key := "g" + strconv.Itoa(id) + "-p" + strconv.Itoa(n) + "@market"
				if err := mgr.AddInstallRecord(key, &config.InstallRecord{
					Scope:       "user",
					InstallPath: "/tmp/cache",
					Version:     "1.0.0",
				}); err != nil {
					errs <- err
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent write error: %v", err)
	}

	// 最终文件必须是合法 JSON 且能解析出全部 goroutines × iterations 条记录。
	installed, err := mgr.LoadInstalledPlugins()
	if err != nil {
		t.Fatalf("final LoadInstalledPlugins failed: %v (state file corrupted by concurrent writes?)", err)
	}
	wantCount := goroutines * iterations
	if len(installed.Plugins) != wantCount {
		t.Errorf("record count = %d, want %d", len(installed.Plugins), wantCount)
	}
}

// TestInstall_SymlinkFailureDoesNotCommitRecord 锁定 P0 修复：
// CreateSymlinksFromManifest 失败时不能写安装记录。
// 之前用 fmt.Printf 把错误吞成 warning，然后照样 AddInstallRecord + 打印"成功"。
func TestInstall_SymlinkFailureDoesNotCommitRecord(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission-based symlink failure cannot be triggered")
	}

	marketPath, _, _, _ := setupUpdateTestMarket(t, "my-plugin", "1.0.0")

	installer, _ := setupInstallerTest(t)
	paths := installer.configMgr.GetPaths()
	skillsDir := filepath.Join(paths.AgentsDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("create skills dir: %v", err)
	}
	// 触发 CreateSymlinksFromManifest 内部的 os.Symlink 失败。
	if err := os.Chmod(skillsDir, 0500); err != nil {
		t.Fatalf("chmod skills dir: %v", err)
	}
	defer os.Chmod(skillsDir, 0755)

	src := &marketplace.LocalMarketSource{Path: marketPath}
	src.SetInstallLocation(marketPath)
	if err := installer.configMgr.AddKnownMarket("test-market", marketplace.MarketSourceToConfig(src)); err != nil {
		t.Fatalf("AddKnownMarket error: %v", err)
	}

	err := installer.Install("my-plugin", InstallOptions{MarketName: "test-market", Scope: "user"})
	if err == nil {
		t.Fatal("expected Install to fail when symlink creation fails, got nil")
	}

	// 关键：不能写安装记录——否则用户以为装好了但其实 symlink / MCP 都没建。
	if _, err := installer.configMgr.GetInstallRecord("my-plugin@test-market"); err == nil {
		t.Errorf("install record must NOT be written when Install fails (false success)")
	}
}
