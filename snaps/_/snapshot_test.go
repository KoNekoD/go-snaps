package _

import (
	"github.com/gkampitakis/go-snaps/internal/configuring"
	"github.com/gkampitakis/go-snaps/snaps"
	"path/filepath"
	"sync"
	"testing"
)

func TestSyncStandaloneRegistry(t *testing.T) {
	t.Run("should increment id on each call [concurrent safe]", func(t *testing.T) {
		wg := sync.WaitGroup{}
		registry := snaps.newStandaloneRegistry()

		for i := 0; i < 5; i++ {
			wg.Add(1)

			go func() {
				registry.getTestID("/file/my_file_%d.snap", "./__snapshots__/my_file_%d.snap")
				wg.Done()
			}()
		}

		wg.Wait()

		snapPath, snapPathRel := registry.getTestID(
			"/file/my_file_%d.snap",
			"./__snapshots__/my_file_%d.snap",
		)

		snaps.Equal(t, "/file/my_file_6.snap", snapPath)
		snaps.Equal(t, "./__snapshots__/my_file_6.snap", snapPathRel)

		snapPath, snapPathRel = registry.getTestID(
			"/file/my_other_file_%d.snap",
			"./__snapshots__/my_other_file_%d.snap",
		)

		snaps.Equal(t, "/file/my_other_file_1.snap", snapPath)
		snaps.Equal(t, "./__snapshots__/my_other_file_1.snap", snapPathRel)
		snaps.Equal(t, registry.cleanup, registry.running)
	})

	t.Run("should reset running registry", func(t *testing.T) {
		wg := sync.WaitGroup{}
		registry := snaps.newStandaloneRegistry()

		for i := 0; i < 100; i++ {
			wg.Add(1)

			go func() {
				registry.getTestID("/file/my_file_%d.snap", "./__snapshots__/my_file_%d.snap")
				wg.Done()
			}()
		}

		wg.Wait()

		registry.reset("/file/my_file_%d.snap")

		snapPath, snapPathRel := registry.getTestID(
			"/file/my_file_%d.snap",
			"./__snapshots__/my_file_%d.snap",
		)

		// running registry start from 0 again
		snaps.Equal(t, "/file/my_file_1.snap", snapPath)
		snaps.Equal(t, "./__snapshots__/my_file_1.snap", snapPathRel)
		// cleanup registry still has 101
		snaps.Equal(t, 101, registry.cleanup["/file/my_file_%d.snap"])
	})
}

func TestAddNewSnapshot(t *testing.T) {
	snapPath := filepath.Join(t.TempDir(), "__snapshots__/mock-test.snap")

	snaps.NoError(t, snaps.addNewSnapshot("my-snap", snapPath))
	snaps.Equal(t, "my-snap", snaps.GetFileContent(t, snapPath))
}

func TestSnapshotPath(t *testing.T) {
	snapshotPathWrapper := func(c *snaps.Config, tName string) (snapPath, snapPathRel string) {
		// This is for emulating being called from a func so we can find the correct file
		// of the caller
		func() {
			func() {
				snapPath, snapPathRel = snaps.snapshotPath(c, tName)
			}()
		}()

		return
	}

	t.Run("should return standalone snapPath", func(t *testing.T) {
		snapPath, snapPathRel := snapshotPathWrapper(&configuring.defaultConfig, "my_test")

		snaps.HasSuffix(
			t,
			snapPath,
			filepath.FromSlash("/snaps/__snapshots__/my_test_%d.snap"),
		)
		snaps.Equal(
			t,
			filepath.FromSlash("__snapshots__/my_test_%d.snap"),
			snapPathRel,
		)
	})

	t.Run("should return standalone snapPath without '/'", func(t *testing.T) {
		snapPath, snapPathRel := snapshotPathWrapper(&configuring.defaultConfig, "TestFunction/my_test")

		snaps.HasSuffix(
			t,
			snapPath,
			filepath.FromSlash("/snaps/__snapshots__/TestFunction_my_test_%d.snap"),
		)
		snaps.Equal(
			t,
			filepath.FromSlash("__snapshots__/TestFunction_my_test_%d.snap"),
			snapPathRel,
		)
	})

	t.Run("should return standalone snapPath with overridden filename", func(t *testing.T) {
		snapPath, snapPathRel := snapshotPathWrapper(snaps.WithConfig(snaps.Filename("my_file"), snaps.Dir("my_snapshot_dir")), "my_test")

		snaps.HasSuffix(t, snapPath, filepath.FromSlash("/snaps/my_snapshot_dir/my_file_%d.snap"))
		snaps.Equal(t, filepath.FromSlash("my_snapshot_dir/my_file_%d.snap"), snapPathRel)
	})

	t.Run(
		"should return standalone snapPath with overridden filename and extension",
		func(t *testing.T) {
			snapPath, snapPathRel := snapshotPathWrapper(snaps.WithConfig(snaps.Filename("my_file"), snaps.Dir("my_snapshot_dir"), snaps.Ext(".txt")), "my_test")

			snaps.HasSuffix(t, snapPath, filepath.FromSlash("/snaps/my_snapshot_dir/my_file_%d.snap.txt"))
			snaps.Equal(t, filepath.FromSlash("my_snapshot_dir/my_file_%d.snap.txt"), snapPathRel)
		},
	)
}

func TestUpdateSnapshot(t *testing.T) {
	const updatedSnap = `

[Test_1/TestSimple - 1]
int(1)
string hello world 1 1 1
---

[Test_3/TestSimple - 1]
int(1250)
string new value
---

[Test_3/TestSimple - 2]
int(1000)
string hello world 1 3 2
---

`
	snapPath := snaps.CreateTempFile(t, _.mockSnap)
	newSnapshot := "int(1250)\nstring new value"

	snaps.NoError(t, snaps.updateSnapshot(newSnapshot, snapPath))
	snaps.Equal(t, updatedSnap, snaps.GetFileContent(t, snapPath))
}

func TestEscapeEndChars(t *testing.T) {
	t.Run("should escape end chars inside data", func(t *testing.T) {
		snapPath := filepath.Join(t.TempDir(), "__snapshots__/mock-test.snap")
		snapshot := TakeSnapshot([]any{"my-snap", snaps.endSequence})

		snaps.NoError(t, snaps.addNewSnapshot(snapshot, snapPath))
		snaps.Equal(t, "\n[mock-id]\nmy-snap\n/-/-/-/\n---\n", snaps.GetFileContent(t, snapPath))
	})

	t.Run("should not escape --- if not end chars", func(t *testing.T) {
		snapPath := filepath.Join(t.TempDir(), "__snapshots__/mock-test.snap")
		snapshot := TakeSnapshot([]any{"my-snap---", snaps.endSequence})

		snaps.NoError(t, snaps.addNewSnapshot(snapshot, snapPath))
		snaps.Equal(t, "\n[mock-id]\nmy-snap---\n/-/-/-/\n---\n", snaps.GetFileContent(t, snapPath))
	})
}
