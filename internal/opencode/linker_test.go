package opencode

import (
	"os"
	"path/filepath"
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

	removed, err := linker.RemoveSymlinks(pluginPath)
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

	removed, err := linker.RemoveSymlinks(pluginPath)
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

	removed, err := linker.RemoveSymlinks(pluginPath)
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
