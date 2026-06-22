package opencode

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func setupLinkerTest(t *testing.T) (*Linker, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	pluginPath := filepath.Join(tmpDir, "plugin")
	os.MkdirAll(agentsDir, 0755)
	os.MkdirAll(pluginPath, 0755)
	linker := NewLinker(agentsDir)
	return linker, agentsDir, pluginPath
}

func TestLinker_CreateSymlinks_DirectoryBased(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	os.MkdirAll(filepath.Join(pluginPath, "skills", "coding"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "coding", "SKILL.md"), []byte("# coding"), 0644)
	os.MkdirAll(filepath.Join(pluginPath, "commands"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "commands", "review.md"), []byte("# review"), 0644)
	os.MkdirAll(filepath.Join(pluginPath, "agents"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "agents", "reviewer.md"), []byte("# reviewer"), 0644)

	counts, err := linker.CreateSymlinksFromManifest(pluginPath, nil, false)
	if err != nil {
		t.Fatalf("CreateSymlinks() error = %v", err)
	}

	if counts.Skills != 1 {
		t.Errorf("Skills = %d, want 1", counts.Skills)
	}
	if counts.Commands != 1 {
		t.Errorf("Commands = %d, want 1", counts.Commands)
	}
	if counts.Agents != 1 {
		t.Errorf("Agents = %d, want 1", counts.Agents)
	}

	assertSymlink(t, filepath.Join(agentsDir, "skills", "coding"), filepath.Join(pluginPath, "skills", "coding"))
	assertSymlink(t, filepath.Join(agentsDir, "commands", "review.md"), filepath.Join(pluginPath, "commands", "review.md"))
	assertSymlink(t, filepath.Join(agentsDir, "agents", "reviewer.md"), filepath.Join(pluginPath, "agents", "reviewer.md"))
}

func TestLinker_CreateSymlinks_NoComponents(t *testing.T) {
	linker, _, pluginPath := setupLinkerTest(t)

	counts, err := linker.CreateSymlinksFromManifest(pluginPath, nil, false)
	if err != nil {
		t.Fatalf("CreateSymlinks() error = %v", err)
	}

	if counts.Skills != 0 {
		t.Errorf("Skills = %d, want 0", counts.Skills)
	}
	if counts.Commands != 0 {
		t.Errorf("Commands = %d, want 0", counts.Commands)
	}
	if counts.Agents != 0 {
		t.Errorf("Agents = %d, want 0", counts.Agents)
	}
}

func TestLinker_CreateSymlinksFromManifest(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	os.MkdirAll(filepath.Join(pluginPath, "skills", "entry-skill"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "entry-skill", "SKILL.md"), []byte("# entry"), 0644)
	os.MkdirAll(filepath.Join(pluginPath, "commands"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "commands", "review.md"), []byte("# review"), 0644)
	os.MkdirAll(filepath.Join(pluginPath, "agents"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "agents", "reviewer.md"), []byte("# reviewer"), 0644)

	manifest := map[string]interface{}{
		"name": "entry-only",
		"skills": []interface{}{
			"./skills/entry-skill",
		},
		"commands": map[string]interface{}{
			"review": map[string]interface{}{
				"source": "./commands/review.md",
			},
		},
		"agents": []interface{}{
			"./agents/reviewer.md",
		},
	}

	counts, err := linker.CreateSymlinksFromManifest(pluginPath, manifest, false)
	if err != nil {
		t.Fatalf("CreateSymlinksFromManifest() error = %v", err)
	}

	if counts.Skills != 1 {
		t.Errorf("Skills = %d, want 1", counts.Skills)
	}
	if counts.Commands != 1 {
		t.Errorf("Commands = %d, want 1", counts.Commands)
	}
	if counts.Agents != 1 {
		t.Errorf("Agents = %d, want 1", counts.Agents)
	}

	assertSymlink(t, filepath.Join(agentsDir, "skills", "entry-skill"), filepath.Join(pluginPath, "skills", "entry-skill"))
	assertSymlink(t, filepath.Join(agentsDir, "commands", "review"), filepath.Join(pluginPath, "commands", "review.md"))
	assertSymlink(t, filepath.Join(agentsDir, "agents", "reviewer.md"), filepath.Join(pluginPath, "agents", "reviewer.md"))
}

func TestLinker_CommandsStringForm(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	os.MkdirAll(filepath.Join(pluginPath, "commands"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "commands", "review.md"), []byte("# review"), 0644)

	manifest := map[string]interface{}{
		"commands": "./commands/review.md",
	}

	counts, err := linker.CreateSymlinksFromManifest(pluginPath, manifest, false)
	if err != nil {
		t.Fatalf("CreateSymlinksFromManifest() error = %v", err)
	}

	if counts.Commands != 1 {
		t.Errorf("Commands = %d, want 1", counts.Commands)
	}

	assertSymlink(t, filepath.Join(agentsDir, "commands", "review"), filepath.Join(pluginPath, "commands", "review.md"))
}

func TestLinker_CommandsArrayForm(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	os.MkdirAll(filepath.Join(pluginPath, "commands"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "commands", "review.md"), []byte("# review"), 0644)
	os.WriteFile(filepath.Join(pluginPath, "commands", "fix.md"), []byte("# fix"), 0644)

	manifest := map[string]interface{}{
		"commands": []interface{}{
			"./commands/review.md",
			"./commands/fix.md",
		},
	}

	counts, err := linker.CreateSymlinksFromManifest(pluginPath, manifest, false)
	if err != nil {
		t.Fatalf("CreateSymlinksFromManifest() error = %v", err)
	}

	if counts.Commands != 2 {
		t.Errorf("Commands = %d, want 2", counts.Commands)
	}

	assertSymlink(t, filepath.Join(agentsDir, "commands", "review"), filepath.Join(pluginPath, "commands", "review.md"))
	assertSymlink(t, filepath.Join(agentsDir, "commands", "fix"), filepath.Join(pluginPath, "commands", "fix.md"))
}

func TestLinker_CommandsInlineContent(t *testing.T) {
	linker, _, pluginPath := setupLinkerTest(t)

	manifest := map[string]interface{}{
		"commands": map[string]interface{}{
			"review": map[string]interface{}{
				"content": "# inline content",
			},
		},
	}

	counts, err := linker.CreateSymlinksFromManifest(pluginPath, manifest, false)
	if err != nil {
		t.Fatalf("CreateSymlinksFromManifest() error = %v", err)
	}

	if counts.Commands != 0 {
		t.Errorf("Commands = %d, want 0 (inline content should be skipped)", counts.Commands)
	}
}

func TestLinker_DefaultComponentDirRejectsSymlinkEscape(t *testing.T) {
	linker, _, pluginPath := setupLinkerTest(t)

	outside := t.TempDir()
	skillsDir := filepath.Join(pluginPath, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(skillsDir, "evil")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	_, err := linker.CreateSymlinksFromManifest(pluginPath, nil, false)
	if err == nil {
		t.Fatal("expected error for symlink escaping plugin root")
	}
}

func TestLinker_RemoveSymlinks(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	os.MkdirAll(filepath.Join(pluginPath, "skills", "coding"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "coding", "SKILL.md"), []byte("# coding"), 0644)
	os.MkdirAll(filepath.Join(pluginPath, "commands"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "commands", "review.md"), []byte("# review"), 0644)
	os.MkdirAll(filepath.Join(pluginPath, "agents"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "agents", "reviewer.md"), []byte("# reviewer"), 0644)

	counts, err := linker.CreateSymlinksFromManifest(pluginPath, nil, false)
	if err != nil {
		t.Fatalf("CreateSymlinks() error = %v", err)
	}

	totalLinked := counts.Skills + counts.Commands + counts.Agents
	if totalLinked != 3 {
		t.Fatalf("expected 3 symlinks created, got %d", totalLinked)
	}

	removed, _, err := linker.RemoveSymlinks(pluginPath, false)
	if err != nil {
		t.Fatalf("RemoveSymlinks() error = %v", err)
	}

	if removed != 3 {
		t.Errorf("removed = %d, want 3", removed)
	}

	assertNoFile(t, filepath.Join(agentsDir, "skills", "coding"))
	assertNoFile(t, filepath.Join(agentsDir, "commands", "review.md"))
	assertNoFile(t, filepath.Join(agentsDir, "agents", "reviewer.md"))
}

func TestLinker_RemoveSymlinks_PreservesOtherLinks(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	otherPluginPath := filepath.Join(filepath.Dir(pluginPath), "other-plugin")
	os.MkdirAll(otherPluginPath, 0755)

	os.MkdirAll(filepath.Join(pluginPath, "skills", "my-skill"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "my-skill", "SKILL.md"), []byte("# mine"), 0644)
	os.MkdirAll(filepath.Join(otherPluginPath, "skills", "other-skill"), 0755)
	os.WriteFile(filepath.Join(otherPluginPath, "skills", "other-skill", "SKILL.md"), []byte("# other"), 0644)

	linker.CreateSymlinksFromManifest(pluginPath, nil, false)
	linker.CreateSymlinksFromManifest(otherPluginPath, nil, false)

	removed, _, err := linker.RemoveSymlinks(pluginPath, false)
	if err != nil {
		t.Fatalf("RemoveSymlinks() error = %v", err)
	}

	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	assertNoFile(t, filepath.Join(agentsDir, "skills", "my-skill"))
	assertSymlink(t, filepath.Join(agentsDir, "skills", "other-skill"), filepath.Join(otherPluginPath, "skills", "other-skill"))
}

func TestLinker_RemoveSymlinks_PreservesNonLinks(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	os.MkdirAll(filepath.Join(pluginPath, "skills", "my-skill"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "my-skill", "SKILL.md"), []byte("# mine"), 0644)

	skillsDir := filepath.Join(agentsDir, "skills")
	os.MkdirAll(skillsDir, 0755)
	os.WriteFile(filepath.Join(skillsDir, "real-file.md"), []byte("# real"), 0644)

	linker.CreateSymlinksFromManifest(pluginPath, nil, false)

	removed, _, err := linker.RemoveSymlinks(pluginPath, false)
	if err != nil {
		t.Fatalf("RemoveSymlinks() error = %v", err)
	}

	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	assertNoFile(t, filepath.Join(agentsDir, "skills", "my-skill"))

	if _, err := os.Stat(filepath.Join(skillsDir, "real-file.md")); err != nil {
		t.Error("real-file.md should not be removed")
	}
}

// TestLinker_RemoveSymlinks_OrphanPreservedWithoutForce reproduces the original bug:
// a symlink whose name matches a plugin entry but whose target lives OUTSIDE the
// plugin cache (typical of dev-time 调试态残留). Without -f the link must survive
// AND be surfaced in warnings — not silently skipped like before.
//
// Also asserts the negative case: a symlink that belongs to ANOTHER plugin (no
// matching name in this plugin's dir) must NOT be flagged as an orphan — that's
// the common, non-orphan case, and flagging it produces false-positive noise.
//
// Warning content (not just count) is asserted, so a regression that inverted
// the (a)/(b) classifier — flagging bar instead of foo — would fail this test.
func TestLinker_RemoveSymlinks_OrphanPreservedWithoutForce(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	// pluginPath 下确实有 skills/foo 这个 entry —— 这是"名字撞车"成立的前提。
	os.MkdirAll(filepath.Join(pluginPath, "skills", "foo"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "foo", "SKILL.md"), []byte("# in-cache"), 0644)

	// orphan target：pluginPath 之外的另一个临时目录，模拟开发态 ln -s 源码仓库。
	outside := filepath.Join(filepath.Dir(pluginPath), "elsewhere")
	os.MkdirAll(outside, 0755)
	os.WriteFile(filepath.Join(outside, "foo"), []byte("# orphan"), 0644)

	skillsDir := filepath.Join(agentsDir, "skills")
	os.MkdirAll(skillsDir, 0755)
	orphanLink := filepath.Join(skillsDir, "foo")
	if err := os.Symlink(outside, orphanLink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// 另一个 plugin 的 skill：本 pluginPath/skills/ 下没有 bar。
	// 它指向外部但名字不撞车 → 必须沉默跳过，不能算 orphan。
	otherPluginPath := filepath.Join(filepath.Dir(pluginPath), "other-plugin")
	os.MkdirAll(filepath.Join(otherPluginPath, "skills", "bar"), 0755)
	os.WriteFile(filepath.Join(otherPluginPath, "skills", "bar", "SKILL.md"), []byte("# other"), 0644)
	otherLink := filepath.Join(skillsDir, "bar")
	if err := os.Symlink(filepath.Join(otherPluginPath, "skills", "bar"), otherLink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	removed, warnings, err := linker.RemoveSymlinks(pluginPath, false)
	if err != nil {
		t.Fatalf("RemoveSymlinks() error = %v", err)
	}
	if removed != 0 {
		t.Errorf("removed = %d, want 0 (orphan must not be deleted without -f)", removed)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %d entries, want 1 (only name-clashing orphan should be surfaced): %v", len(warnings), warnings)
	}
	// 内容断言：warning 必须提到 orphan foo，且不能误带 bar。
	// 防止 (a)/(b) 分类被反转后仍然 count=1 通过测试。
	w := warnings[0]
	if !strings.Contains(w, "foo") {
		t.Errorf("warning should mention the orphan 'foo', got: %s", w)
	}
	if strings.Contains(w, "bar") {
		t.Errorf("warning must NOT mention other plugin's 'bar' (false positive), got: %s", w)
	}

	// orphan symlink 仍在 —— 这是非 force 模式的核心约定。
	if _, err := os.Lstat(orphanLink); err != nil {
		t.Errorf("orphan symlink %s should still exist (got %v)", orphanLink, err)
	}
	// 别人的 symlink 也必须仍在，并且没有被误报。
	if _, err := os.Lstat(otherLink); err != nil {
		t.Errorf("other plugin's symlink %s should still exist (got %v)", otherLink, err)
	}
}

// TestLinker_RemoveSymlinks_OrphanForceRemoved verifies -f deletes the orphan
// and counts it as removed, matching the name-based semantics of linkComponentDir
// on the creation side. Also asserts no warnings under force and that an
// other-plugin symlink survives (force only targets name-clashing orphans).
func TestLinker_RemoveSymlinks_OrphanForceRemoved(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	os.MkdirAll(filepath.Join(pluginPath, "skills", "foo"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "foo", "SKILL.md"), []byte("# in-cache"), 0644)

	outside := filepath.Join(filepath.Dir(pluginPath), "elsewhere")
	os.MkdirAll(outside, 0755)
	os.WriteFile(filepath.Join(outside, "foo"), []byte("# orphan"), 0644)

	skillsDir := filepath.Join(agentsDir, "skills")
	os.MkdirAll(skillsDir, 0755)
	orphanLink := filepath.Join(skillsDir, "foo")
	if err := os.Symlink(outside, orphanLink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// 别人的 symlink 也在 —— force 不能误删。
	otherPluginPath := filepath.Join(filepath.Dir(pluginPath), "other-plugin")
	os.MkdirAll(filepath.Join(otherPluginPath, "skills", "bar"), 0755)
	os.WriteFile(filepath.Join(otherPluginPath, "skills", "bar", "SKILL.md"), []byte("# other"), 0644)
	otherLink := filepath.Join(skillsDir, "bar")
	if err := os.Symlink(filepath.Join(otherPluginPath, "skills", "bar"), otherLink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	removed, warnings, err := linker.RemoveSymlinks(pluginPath, true)
	if err != nil {
		t.Fatalf("RemoveSymlinks() error = %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1 (force should delete the orphan only)", removed)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %d entries, want 0 under force (got %v)", len(warnings), warnings)
	}

	assertNoFile(t, orphanLink)
	// force 不能误伤别人的 symlink。
	if _, err := os.Lstat(otherLink); err != nil {
		t.Errorf("other plugin's symlink %s should survive force (got %v)", otherLink, err)
	}
}

// TestLinker_RemoveSymlinks_MixedInCacheAndOrphan verifies a single RemoveSymlinks
// call correctly handles a mix of in-cache (delete) and orphan (warn/keep) entries
// in the same loop. Guards against a bug that returned early on the first orphan
// instead of continuing to process subsequent in-cache entries.
func TestLinker_RemoveSymlinks_MixedInCacheAndOrphan(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	// foo: in-cache symlink (should be removed)
	os.MkdirAll(filepath.Join(pluginPath, "skills", "foo"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "foo", "SKILL.md"), []byte("# in-cache"), 0644)
	// bar: orphan — name exists in plugin but target points outside
	os.MkdirAll(filepath.Join(pluginPath, "skills", "bar"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "bar", "SKILL.md"), []byte("# in-cache-bar"), 0644)
	// baz: in-cache symlink (should be removed) — comes AFTER the orphan
	// to verify the loop doesn't early-return on the orphan.
	os.MkdirAll(filepath.Join(pluginPath, "skills", "baz"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "baz", "SKILL.md"), []byte("# in-cache-baz"), 0644)

	skillsDir := filepath.Join(agentsDir, "skills")
	os.MkdirAll(skillsDir, 0755)

	// in-cache links
	mustSymlink(t, filepath.Join(pluginPath, "skills", "foo"), filepath.Join(skillsDir, "foo"))
	mustSymlink(t, filepath.Join(pluginPath, "skills", "baz"), filepath.Join(skillsDir, "baz"))
	// orphan link (target outside)
	outside := filepath.Join(filepath.Dir(pluginPath), "elsewhere")
	os.MkdirAll(outside, 0755)
	os.WriteFile(filepath.Join(outside, "bar"), []byte("# orphan"), 0644)
	mustSymlink(t, outside, filepath.Join(skillsDir, "bar"))

	t.Run("force=false", func(t *testing.T) {
		removed, warnings, err := linker.RemoveSymlinks(pluginPath, false)
		if err != nil {
			t.Fatalf("RemoveSymlinks() error = %v", err)
		}
		// foo + baz deleted; bar warned.
		if removed != 2 {
			t.Errorf("removed = %d, want 2 (foo + baz, both in-cache)", removed)
		}
		if len(warnings) != 1 {
			t.Fatalf("warnings = %d, want 1 (bar only): %v", len(warnings), warnings)
		}
		if !strings.Contains(warnings[0], "bar") {
			t.Errorf("warning should mention orphan 'bar', got: %s", warnings[0])
		}
		// bar survives (no force)
		if _, err := os.Lstat(filepath.Join(skillsDir, "bar")); err != nil {
			t.Errorf("orphan 'bar' should survive without force: %v", err)
		}
	})
}

// TestLinker_RemoveSymlinks_NameClashPathForUpdate simulates Update Stage 2's
// exact situation: the original plugin cache dir has been renamed to *.update-backup
// before RemoveSymlinks runs. The default pluginPath lookup would always miss the
// entry (IsNotExist) and silently skip orphans; the nameClashPath parameter
// routes the lookup to the backup dir so orphans are correctly detected.
func TestLinker_RemoveSymlinks_NameClashPathForUpdate(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	// 在 rename 之前先把 skills/foo 建好——rename 之后它就跟着到了 backup 里。
	os.MkdirAll(filepath.Join(pluginPath, "skills", "foo"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "foo", "SKILL.md"), []byte("# in-cache"), 0644)

	// 模拟 Stage 1：把原始 cache rename 成 .update-backup。
	backupPath := pluginPath + ".update-backup"
	if err := os.Rename(pluginPath, backupPath); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// orphan symlink：名字 foo 在 backup 里有，target 在外。
	outside := filepath.Join(filepath.Dir(pluginPath), "elsewhere")
	os.MkdirAll(outside, 0755)
	os.WriteFile(filepath.Join(outside, "foo"), []byte("# orphan"), 0644)
	skillsDir := filepath.Join(agentsDir, "skills")
	os.MkdirAll(skillsDir, 0755)
	orphanLink := filepath.Join(skillsDir, "foo")
	mustSymlink(t, outside, orphanLink)

	t.Run("default_path_silently_misses_orphan", func(t *testing.T) {
		// 不传 nameClashPath：pluginPath 不存在 → srcEntry IsNotExist → 沉默跳过。
		// 这是 Update Stage 2 修复前的行为；保留作为回归对照。
		removed, warnings, err := linker.RemoveSymlinks(pluginPath, false)
		if err != nil {
			t.Fatalf("RemoveSymlinks() error = %v", err)
		}
		if removed != 0 || len(warnings) != 0 {
			t.Errorf("default path should miss orphan (pluginPath gone): removed=%d warnings=%v", removed, warnings)
		}
		// 重置：把 orphan link 重建（上面那次调用没动它）。
		if _, err := os.Lstat(orphanLink); err != nil {
			mustSymlink(t, outside, orphanLink)
		}
	})

	t.Run("nameClashPath_finds_orphan_in_backup", func(t *testing.T) {
		// 传 backupPath 作为 nameClashPath：现在能查到 foo → 正确识别 orphan。
		removed, warnings, err := linker.RemoveSymlinksWithNameClashPath(pluginPath, backupPath, false)
		if err != nil {
			t.Fatalf("RemoveSymlinksWithNameClashPath() error = %v", err)
		}
		if removed != 0 {
			t.Errorf("removed = %d, want 0 (non-force keeps orphan)", removed)
		}
		if len(warnings) != 1 {
			t.Fatalf("warnings = %d, want 1 (orphan should be detected via backupPath): %v", len(warnings), warnings)
		}
		if !strings.Contains(warnings[0], "foo") {
			t.Errorf("warning should mention 'foo', got: %s", warnings[0])
		}
	})

	t.Run("nameClashPath_with_force_deletes_orphan", func(t *testing.T) {
		// 确保 orphan 还在（上一个 subtest 没 force 删）。
		if _, err := os.Lstat(orphanLink); err != nil {
			mustSymlink(t, outside, orphanLink)
		}
		removed, warnings, err := linker.RemoveSymlinksWithNameClashPath(pluginPath, backupPath, true)
		if err != nil {
			t.Fatalf("RemoveSymlinksWithNameClashPath() error = %v", err)
		}
		if removed != 1 {
			t.Errorf("removed = %d, want 1 (force should delete orphan)", removed)
		}
		if len(warnings) != 0 {
			t.Errorf("warnings = %d, want 0 under force: %v", len(warnings), warnings)
		}
		assertNoFile(t, orphanLink)
	})
}

// TestLinker_RemoveSymlinks_NormalInCache regression-guards the happy path under
// both force values: a normal in-cache symlink is removed regardless of -f.
func TestLinker_RemoveSymlinks_NormalInCache(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run("force="+strconv.FormatBool(force), func(t *testing.T) {
			linker, agentsDir, pluginPath := setupLinkerTest(t)

			os.MkdirAll(filepath.Join(pluginPath, "skills", "foo"), 0755)
			os.WriteFile(filepath.Join(pluginPath, "skills", "foo", "SKILL.md"), []byte("# in-cache"), 0644)

			if _, err := linker.CreateSymlinksFromManifest(pluginPath, nil, false); err != nil {
				t.Fatalf("CreateSymlinks() error = %v", err)
			}

			linkPath := filepath.Join(agentsDir, "skills", "foo")
			if _, err := os.Lstat(linkPath); err != nil {
				t.Fatalf("setup: symlink not created: %v", err)
			}

			removed, warnings, err := linker.RemoveSymlinks(pluginPath, force)
			if err != nil {
				t.Fatalf("RemoveSymlinks() error = %v", err)
			}
			if removed != 1 {
				t.Errorf("removed = %d, want 1", removed)
			}
			if len(warnings) != 0 {
				t.Errorf("warnings = %d entries, want 0 for in-cache symlink (got %v)", len(warnings), warnings)
			}
			assertNoFile(t, linkPath)
		})
	}
}

func TestLinker_CreateSymlinks_Idempotent(t *testing.T) {
	linker, _, pluginPath := setupLinkerTest(t)

	os.MkdirAll(filepath.Join(pluginPath, "skills", "coding"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "coding", "SKILL.md"), []byte("# coding"), 0644)

	counts1, err := linker.CreateSymlinksFromManifest(pluginPath, nil, false)
	if err != nil {
		t.Fatalf("first CreateSymlinks() error = %v", err)
	}

	counts2, err := linker.CreateSymlinksFromManifest(pluginPath, nil, false)
	if err != nil {
		t.Fatalf("second CreateSymlinks() error = %v", err)
	}

	if counts1.Skills != counts2.Skills {
		t.Errorf("idempotent check: first=%d, second=%d", counts1.Skills, counts2.Skills)
	}
}

func TestLinker_CreateSymlinks_ForceOverwrite(t *testing.T) {
	linker, agentsDir, pluginPath := setupLinkerTest(t)

	// Create first plugin with skill "coding"
	os.MkdirAll(filepath.Join(pluginPath, "skills", "coding"), 0755)
	os.WriteFile(filepath.Join(pluginPath, "skills", "coding", "SKILL.md"), []byte("# coding v1"), 0644)

	counts1, err := linker.CreateSymlinksFromManifest(pluginPath, nil, false)
	if err != nil {
		t.Fatalf("first CreateSymlinks() error = %v", err)
	}
	if counts1.Skills != 1 {
		t.Fatalf("expected 1 skill linked, got %d", counts1.Skills)
	}

	// Simulate another plugin trying to overwrite the same skill name
	otherPluginPath := filepath.Join(filepath.Dir(pluginPath), "other-plugin")
	os.MkdirAll(otherPluginPath, 0755)
	os.MkdirAll(filepath.Join(otherPluginPath, "skills", "coding"), 0755)
	os.WriteFile(filepath.Join(otherPluginPath, "skills", "coding", "SKILL.md"), []byte("# coding v2"), 0644)

	// Without force, should skip
	linker2 := NewLinker(agentsDir)
	counts2, err := linker2.CreateSymlinksFromManifest(otherPluginPath, nil, false)
	if err != nil {
		t.Fatalf("second CreateSymlinks() error = %v", err)
	}
	if counts2.Skills != 0 {
		t.Fatalf("expected 0 skills linked (conflict), got %d", counts2.Skills)
	}

	// With force, should overwrite
	counts3, err := linker2.CreateSymlinksFromManifest(otherPluginPath, nil, true)
	if err != nil {
		t.Fatalf("force CreateSymlinksFromManifest() error = %v", err)
	}
	if counts3.Skills != 1 {
		t.Fatalf("expected 1 skill linked with force, got %d", counts3.Skills)
	}

	// Verify symlink now points to other plugin
	assertSymlink(t, filepath.Join(agentsDir, "skills", "coding"), filepath.Join(otherPluginPath, "skills", "coding"))
}

func assertSymlink(t *testing.T, linkPath, expectedTarget string) {
	t.Helper()

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Errorf("symlink %s does not exist: %v", linkPath, err)
		return
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("%s is not a symlink", linkPath)
		return
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Errorf("failed to read symlink %s: %v", linkPath, err)
		return
	}

	absTarget, _ := filepath.Abs(target)
	absExpected, _ := filepath.Abs(expectedTarget)
	evTarget, _ := filepath.EvalSymlinks(absTarget)
	if evTarget == "" {
		evTarget = absTarget
	}
	evExpected, _ := filepath.EvalSymlinks(absExpected)
	if evExpected == "" {
		evExpected = absExpected
	}
	if evTarget != evExpected {
		t.Errorf("symlink %s target = %s, want %s", linkPath, evTarget, evExpected)
	}
}

func assertNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("file %s should not exist", path)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
}
