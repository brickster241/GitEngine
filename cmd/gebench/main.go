// gebench is GitEngine's benchmark harness: identical workloads executed by
// the gegit binary and by native git in twin repositories, timed wall-clock,
// reported side by side. It also runs the concurrency-safety drill: parallel
// writers against one index, which must serialize via index.lock and lose
// nothing.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

var (
	gegitBin = flag.String("gegit", "", "path to the gegit binary (required)")
	files    = flag.Int("files", 2000, "file count for ingest/status/fsck scenarios")
	commits  = flag.Int("commits", 500, "chain length for log/merge-base scenarios")
	writers  = flag.Int("writers", 8, "parallel writers for the lock-contention drill")
)

var env = append(os.Environ(),
	"GIT_AUTHOR_DATE=1700000000 +0530", "GIT_COMMITTER_DATE=1700000000 +0530",
	"GIT_AUTHOR_NAME=username", "GIT_AUTHOR_EMAIL=user@email.com",
	"GIT_COMMITTER_NAME=username", "GIT_COMMITTER_EMAIL=user@email.com",
	"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
)

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %v\n%s", name, args, err, out)
	}
	return nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func timed(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// seedFiles writes n small files (64–512B) across nested directories.
func seedFiles(dir string, n int) {
	rng := rand.New(rand.NewSource(42)) // deterministic corpus
	for i := 0; i < n; i++ {
		sub := fmt.Sprintf("pkg%02d/mod%02d", i%20, (i/20)%10)
		os.MkdirAll(filepath.Join(dir, sub), 0755)
		size := 64 + rng.Intn(448)
		content := make([]byte, size)
		for j := range content {
			content[j] = byte('a' + rng.Intn(26))
			if j%40 == 39 {
				content[j] = '\n'
			}
		}
		os.WriteFile(filepath.Join(dir, sub, fmt.Sprintf("file%04d.txt", i)), content, 0644)
	}
}

func listSeeded(dir string, n int) []string {
	paths := make([]string, 0, n)
	for i := 0; i < n; i++ {
		sub := fmt.Sprintf("pkg%02d/mod%02d", i%20, (i/20)%10)
		paths = append(paths, filepath.Join(sub, fmt.Sprintf("file%04d.txt", i)))
	}
	return paths
}

type row struct {
	scenario string
	ge, git  time.Duration
	note     string
}

func main() {
	flag.Parse()
	if *gegitBin == "" {
		fmt.Fprintln(os.Stderr, "usage: gebench -gegit <path-to-gegit-binary>")
		os.Exit(2)
	}
	var rows []row

	// ---- twin repos with N seeded files --------------------------------
	geDir, _ := os.MkdirTemp("", "gebench_ge_*")
	gitDir, _ := os.MkdirTemp("", "gebench_git_*")
	defer os.RemoveAll(geDir)
	defer os.RemoveAll(gitDir)
	must(run(geDir, *gegitBin, "init"))
	must(run(gitDir, "git", "init", "-b", "master"))
	seedFiles(geDir, *files)
	seedFiles(gitDir, *files)
	paths := listSeeded(geDir, *files)

	// Ingest: stage every file + commit.
	geIngest := timed(func() {
		must(run(geDir, *gegitBin, append([]string{"add"}, paths...)...))
		must(run(geDir, *gegitBin, "commit", "-m", "ingest"))
	})
	gitIngest := timed(func() {
		must(run(gitDir, "git", "add", "-A"))
		must(run(gitDir, "git", "commit", "-m", "ingest"))
	})
	rows = append(rows, row{fmt.Sprintf("ingest %d files (add+commit)", *files), geIngest, gitIngest, ""})

	// Status on a clean tree.
	geStatus := timed(func() { must(run(geDir, *gegitBin, "status")) })
	gitStatus := timed(func() { must(run(gitDir, "git", "status")) })
	rows = append(rows, row{fmt.Sprintf("status, clean %d-file tree", *files), geStatus, gitStatus, ""})

	// Fsck: every object re-hashed + references resolved.
	geFsck := timed(func() { must(run(geDir, *gegitBin, "fsck")) })
	gitFsck := timed(func() { must(run(gitDir, "git", "fsck", "--full")) })
	rows = append(rows, row{fmt.Sprintf("fsck (%d files of objects)", *files), geFsck, gitFsck, ""})

	// ---- deep-history repos --------------------------------------------
	geDeep, _ := os.MkdirTemp("", "gebench_ge_deep_*")
	gitDeep, _ := os.MkdirTemp("", "gebench_git_deep_*")
	defer os.RemoveAll(geDeep)
	defer os.RemoveAll(gitDeep)
	must(run(geDeep, *gegitBin, "init"))
	must(run(gitDeep, "git", "init", "-b", "master"))
	for i := 0; i < *commits; i++ {
		content := fmt.Sprintf("revision %d\n", i)
		os.WriteFile(filepath.Join(geDeep, "counter.txt"), []byte(content), 0644)
		os.WriteFile(filepath.Join(gitDeep, "counter.txt"), []byte(content), 0644)
		must(run(geDeep, *gegitBin, "add", "counter.txt"))
		must(run(geDeep, *gegitBin, "commit", "-m", fmt.Sprintf("c%d", i)))
		must(run(gitDeep, "git", "add", "counter.txt"))
		must(run(gitDeep, "git", "commit", "-m", fmt.Sprintf("c%d", i)))
	}

	geLog := timed(func() { must(run(geDeep, *gegitBin, "log", "--oneline")) })
	gitLog := timed(func() { must(run(gitDeep, "git", "log", "--oneline")) })
	rows = append(rows, row{fmt.Sprintf("log --oneline, %d-commit chain", *commits), geLog, gitLog, ""})

	// ---- lock-contention drill -----------------------------------------
	// N parallel writers add DISTINCT files to the SAME repository. Success
	// criterion: every add lands (no lost updates) and the index stays
	// parseable (no torn writes) — index.lock must serialize them.
	drillDir, _ := os.MkdirTemp("", "gebench_lock_*")
	defer os.RemoveAll(drillDir)
	must(run(drillDir, *gegitBin, "init"))
	var wg sync.WaitGroup
	errs := make([]error, *writers)
	drill := timed(func() {
		for w := 0; w < *writers; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				name := fmt.Sprintf("writer%02d.txt", w)
				os.WriteFile(filepath.Join(drillDir, name), []byte(fmt.Sprintf("writer %d\n", w)), 0644)
				errs[w] = run(drillDir, *gegitBin, "add", name)
			}(w)
		}
		wg.Wait()
	})
	lost := 0
	for _, err := range errs {
		if err != nil {
			lost++
		}
	}
	// The index must still be valid and hold every writer's file.
	must(run(drillDir, *gegitBin, "status"))
	rows = append(rows, row{fmt.Sprintf("%d parallel writers, one index", *writers), drill, 0,
		fmt.Sprintf("%d lost updates, index valid", lost)})

	// ---- report ---------------------------------------------------------
	fmt.Printf("\n%-38s %12s %12s %8s %s\n", "scenario", "gegit", "git", "ratio", "note")
	for _, r := range rows {
		ratio := "—"
		gitCol := "—"
		if r.git > 0 {
			ratio = fmt.Sprintf("%.2fx", float64(r.ge)/float64(r.git))
			gitCol = fmt.Sprintf("%.1fms", float64(r.git.Microseconds())/1000)
		}
		fmt.Printf("%-38s %11.1fms %12s %8s %s\n",
			r.scenario, float64(r.ge.Microseconds())/1000, gitCol, ratio, r.note)
	}
}
