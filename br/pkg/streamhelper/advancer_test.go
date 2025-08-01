// Copyright 2022 PingCAP, Inc. Licensed under Apache-2.0.

package streamhelper_test

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/failpoint"
	backup "github.com/pingcap/kvproto/pkg/brpb"
	logbackup "github.com/pingcap/kvproto/pkg/logbackuppb"
	"github.com/pingcap/log"
	"github.com/pingcap/tidb/br/pkg/logutil"
	"github.com/pingcap/tidb/br/pkg/streamhelper"
	"github.com/pingcap/tidb/br/pkg/streamhelper/config"
	"github.com/pingcap/tidb/br/pkg/streamhelper/spans"
	"github.com/pingcap/tidb/pkg/kv"
	"github.com/pingcap/tidb/pkg/util/redact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikv/client-go/v2/oracle"
	"github.com/tikv/client-go/v2/tikv"
	"github.com/tikv/client-go/v2/txnkv/txnlock"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBasic(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		if t.Failed() {
			fmt.Println(c)
		}
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx := context.Background()
	minCheckpoint := c.advanceCheckpoints()
	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	coll := streamhelper.NewClusterCollector(ctx, env)
	err := adv.GetCheckpointInRange(ctx, []byte{}, []byte{}, coll)
	require.NoError(t, err)
	r, err := coll.Finish(ctx)
	require.NoError(t, err)
	require.Len(t, r.FailureSubRanges, 0)
	require.Equal(t, r.Checkpoint, minCheckpoint, "%d %d", r.Checkpoint, minCheckpoint)
}

func TestTick(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)
	require.NoError(t, adv.OnTick(ctx))
	for range 5 {
		cp := c.advanceCheckpoints()
		require.NoError(t, adv.OnTick(ctx))
		require.Equal(t, env.getCheckpoint(), cp)
	}
}

func TestWithFailure(t *testing.T) {
	logutil.OverrideLevelForTest(t, zapcore.DebugLevel)
	c := createFakeCluster(t, 4, true)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	c.flushAll()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)
	require.NoError(t, adv.OnTick(ctx))

	cp := c.advanceCheckpoints()
	for _, v := range c.stores {
		v.flush()
		break
	}
	require.NoError(t, adv.OnTick(ctx))
	require.Less(t, env.getCheckpoint(), cp, "%d %d", env.getCheckpoint(), cp)

	for _, v := range c.stores {
		v.flush()
	}

	require.NoError(t, adv.OnTick(ctx))
	require.Equal(t, env.getCheckpoint(), cp)
}

func shouldFinishInTime(t *testing.T, d time.Duration, name string, f func()) {
	ch := make(chan struct{})
	go func() {
		f()
		close(ch)
	}()
	select {
	case <-time.After(d):
		t.Fatalf("%s should finish in %s, but not", name, d)
	case <-ch:
	}
}

func TestCollectorFailure(t *testing.T) {
	logutil.OverrideLevelForTest(t, zapcore.DebugLevel)
	c := createFakeCluster(t, 4, true)
	c.onGetClient = func(u uint64) error {
		return status.Error(codes.DataLoss,
			"Exiled requests from the client, please slow down and listen a story: "+
				"the server has been dropped, we are longing for new nodes, however the goddess(k8s) never allocates new resource. "+
				"May you take the sword named `vim`, refactoring the definition of the nature, in the yaml file hidden at somewhere of the cluster, "+
				"to save all of us and gain the response you desiring?")
	}
	ctx := context.Background()
	splitKeys := make([]string, 0, 10000)
	for i := range 10000 {
		splitKeys = append(splitKeys, fmt.Sprintf("%04d", i))
	}
	c.splitAndScatter(splitKeys...)

	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	coll := streamhelper.NewClusterCollector(ctx, env)

	shouldFinishInTime(t, 30*time.Second, "scan with always fail", func() {
		// At this time, the sending may or may not fail because the sending and batching is doing asynchronously.
		_ = adv.GetCheckpointInRange(ctx, []byte{}, []byte{}, coll)
		// ...but this must fail, not getting stuck.
		_, err := coll.Finish(ctx)
		require.Error(t, err)
	})
}

func oneStoreFailure() func(uint64) error {
	victim := uint64(0)
	mu := new(sync.Mutex)
	return func(u uint64) error {
		mu.Lock()
		defer mu.Unlock()
		if victim == 0 {
			victim = u
		}
		if victim == u {
			return status.Error(codes.NotFound,
				"The place once lit by the warm lamplight has been swallowed up by the debris now.")
		}
		return nil
	}
}

func TestOneStoreFailure(t *testing.T) {
	logutil.OverrideLevelForTest(t, zapcore.DebugLevel)
	c := createFakeCluster(t, 4, true)
	ctx := context.Background()
	splitKeys := make([]string, 0, 1000)
	for i := range 1000 {
		splitKeys = append(splitKeys, fmt.Sprintf("%04d", i))
	}
	c.splitAndScatter(splitKeys...)
	c.flushAll()

	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)
	require.NoError(t, adv.OnTick(ctx))
	c.onGetClient = oneStoreFailure()

	for range 100 {
		c.advanceCheckpoints()
		c.flushAll()
		require.ErrorContains(t, adv.OnTick(ctx), "the warm lamplight")
	}

	c.onGetClient = nil
	cp := c.advanceCheckpoints()
	c.flushAll()
	require.NoError(t, adv.OnTick(ctx))
	require.Equal(t, cp, env.checkpoint)
}

func TestGCServiceSafePoint(t *testing.T) {
	req := require.New(t)
	c := createFakeCluster(t, 4, true)
	ctx := context.Background()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	env := newTestEnv(c, t)

	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)
	cp := c.advanceCheckpoints()
	c.flushAll()

	req.NoError(adv.OnTick(ctx))
	req.Equal(env.serviceGCSafePoint, cp-1)

	env.unregisterTask()
	req.Eventually(func() bool {
		env.fakeCluster.mu.Lock()
		defer env.fakeCluster.mu.Unlock()
		return env.serviceGCSafePoint != 0 && env.serviceGCSafePointDeleted
	}, 3*time.Second, 100*time.Millisecond)
}

func TestTaskRanges(t *testing.T) {
	logutil.OverrideLevelForTest(t, zapcore.DebugLevel)
	c := createFakeCluster(t, 4, true)
	defer fmt.Println(c)
	ctx := context.Background()
	c.splitAndScatter("0001", "0002", "0012", "0034", "0048")
	c.advanceCheckpoints()
	c.flushAllExcept("0000", "0049")
	env := newTestEnv(c, t)
	env.ranges = []kv.KeyRange{{StartKey: []byte("0002"), EndKey: []byte("0048")}}
	env.task.Ranges = env.ranges
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)

	shouldFinishInTime(t, 10*time.Second, "first advancing", func() { require.NoError(t, adv.OnTick(ctx)) })
	// Don't check the return value of advance checkpoints here -- we didn't
	require.Greater(t, env.getCheckpoint(), uint64(0))
}

func TestTaskRangesWithSplit(t *testing.T) {
	logutil.OverrideLevelForTest(t, zapcore.DebugLevel)
	c := createFakeCluster(t, 4, true)
	defer fmt.Println(c)
	ctx := context.Background()
	c.splitAndScatter("0012", "0034", "0048")
	c.advanceCheckpoints()
	c.flushAllExcept("0049")
	env := newTestEnv(c, t)
	env.ranges = []kv.KeyRange{{StartKey: []byte("0002"), EndKey: []byte("0048")}}
	env.task.Ranges = env.ranges
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)

	shouldFinishInTime(t, 10*time.Second, "first advancing", func() { require.NoError(t, adv.OnTick(ctx)) })
	fstCheckpoint := env.getCheckpoint()
	require.Greater(t, fstCheckpoint, uint64(0))

	c.splitAndScatter("0002")
	c.advanceCheckpoints()
	c.flushAllExcept("0000", "0049")
	shouldFinishInTime(t, 10*time.Second, "second advancing", func() { require.NoError(t, adv.OnTick(ctx)) })
	require.Greater(t, env.getCheckpoint(), fstCheckpoint)
}

func TestClearCache(t *testing.T) {
	c := createFakeCluster(t, 4, true)
	ctx := context.Background()
	req := require.New(t)
	c.splitAndScatter("0012", "0034", "0048")

	clearedCache := make(map[uint64]bool)
	c.onClearCache = func(u uint64) error {
		// make store u cache cleared
		clearedCache[u] = true
		return nil
	}
	failedStoreID := uint64(0)
	hasFailed := atomic.NewBool(false)
	for _, s := range c.stores {
		s.clientMu.Lock()
		sid := s.GetID()
		s.onGetRegionCheckpoint = func(glftrr *logbackup.GetLastFlushTSOfRegionRequest) error {
			// mark one store failed is enough
			if hasFailed.CompareAndSwap(false, true) {
				// mark this store cache cleared
				failedStoreID = sid
				return errors.New("failed to get checkpoint")
			}
			return nil
		}
		s.clientMu.Unlock()
	}
	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)
	var err error
	shouldFinishInTime(t, time.Second, "ticking", func() {
		err = adv.OnTick(ctx)
	})
	req.Error(err)
	req.True(failedStoreID > 0, "failed to mark the cluster: ")
	req.Equal(clearedCache[failedStoreID], true)
}

func TestBlocked(t *testing.T) {
	logutil.OverrideLevelForTest(t, zapcore.DebugLevel)
	c := createFakeCluster(t, 4, true)
	ctx := context.Background()
	req := require.New(t)
	c.splitAndScatter("0012", "0034", "0048")
	marked := false
	for _, s := range c.stores {
		s.clientMu.Lock()
		s.onGetRegionCheckpoint = func(glftrr *logbackup.GetLastFlushTSOfRegionRequest) error {
			// blocking the thread.
			// this may happen when TiKV goes down or too busy.
			<-(chan struct{})(nil)
			return nil
		}
		s.clientMu.Unlock()
		marked = true
	}
	req.True(marked, "failed to mark the cluster: ")
	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)
	adv.UpdateConfigWith(func(c *config.CommandConfig) {
		// ... So the tick timeout would be 100ms
		c.TickDuration = 10 * time.Millisecond
	})
	var err error
	shouldFinishInTime(t, time.Second, "ticking", func() {
		err = adv.OnTick(ctx)
	})
	req.ErrorIs(errors.Cause(err), context.DeadlineExceeded)
}

func TestResolveLock(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		if t.Failed() {
			fmt.Println(c)
		}
	}()
	require.NoError(t, failpoint.Enable("github.com/pingcap/tidb/br/pkg/streamhelper/NeedResolveLocks", `return(true)`))
	// make sure asyncResolveLocks stuck in optionalTick later.
	require.NoError(t, failpoint.Enable("github.com/pingcap/tidb/br/pkg/streamhelper/AsyncResolveLocks", `pause`))
	defer func() {
		require.NoError(t, failpoint.Disable("github.com/pingcap/tidb/br/pkg/streamhelper/NeedResolveLocks"))
	}()

	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx := context.Background()
	minCheckpoint := c.advanceCheckpoints()
	env := newTestEnv(c, t)

	lockRegion := c.findRegionByKey([]byte("01"))
	allLocks := []*txnlock.Lock{
		{
			Key: []byte("011"),
			// TxnID == minCheckpoint
			TxnID: minCheckpoint,
		},
		{
			Key: []byte("012"),
			// TxnID > minCheckpoint
			TxnID: minCheckpoint + 1,
		},
		{
			Key: []byte("013"),
			// this lock cannot be resolved due to scan version
			TxnID: oracle.GoTimeToTS(oracle.GetTimeFromTS(minCheckpoint).Add(2 * time.Minute)),
		},
	}
	c.LockRegion(lockRegion, allLocks)

	// ensure resolve locks triggered and collect all locks from scan locks
	resolveLockRef := atomic.NewBool(false)
	env.resolveLocks = func(locks []*txnlock.Lock, loc *tikv.KeyLocation) (*tikv.KeyLocation, error) {
		resolveLockRef.Store(true)
		// The third lock has skipped, because it's less than max version.
		require.ElementsMatch(t, locks, allLocks[:2])
		return loc, nil
	}
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)

	maxTargetTs := uint64(0)
	coll := streamhelper.NewClusterCollector(ctx, env)
	coll.SetOnSuccessHook(func(u uint64, kr kv.KeyRange) {
		adv.WithCheckpoints(func(s *spans.ValueSortedFull) {
			for _, lock := range allLocks {
				// if there is any lock key in the range
				if bytes.Compare(kr.StartKey, lock.Key) <= 0 && (bytes.Compare(lock.Key, kr.EndKey) < 0 || len(kr.EndKey) == 0) {
					// mock lock behavior, do not update checkpoint
					s.Merge(spans.Valued{Key: kr, Value: minCheckpoint})
					return
				}
			}
			s.Merge(spans.Valued{Key: kr, Value: u})
			maxTargetTs = max(maxTargetTs, u)
		})
	})
	err := adv.GetCheckpointInRange(ctx, []byte{}, []byte{}, coll)
	require.NoError(t, err)
	r, err := coll.Finish(ctx)
	require.NoError(t, err)
	require.Len(t, r.FailureSubRanges, 0)
	require.Equal(t, r.Checkpoint, minCheckpoint, "%d %d", r.Checkpoint, minCheckpoint)

	env.maxTs = maxTargetTs + 1
	require.Eventually(t, func() bool { return adv.OnTick(ctx) == nil },
		time.Second, 50*time.Millisecond)
	// now the lock state must be ture. because tick finished and asyncResolveLocks got stuck.
	require.True(t, adv.GetInResolvingLock())
	require.NoError(t, failpoint.Disable("github.com/pingcap/tidb/br/pkg/streamhelper/AsyncResolveLocks"))
	require.Eventually(t, func() bool { return resolveLockRef.Load() },
		8*time.Second, 50*time.Microsecond)
	// state must set to false after tick
	require.Eventually(t, func() bool { return !adv.GetInResolvingLock() },
		8*time.Second, 50*time.Microsecond)
}

func TestOwnerDropped(t *testing.T) {
	ctx := context.Background()
	c := createFakeCluster(t, 4, false)
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	installSubscribeSupport(c)
	env := newTestEnv(c, t)
	fp := "github.com/pingcap/tidb/br/pkg/streamhelper/get_subscriber"
	defer func() {
		if t.Failed() {
			fmt.Println(c)
		}
	}()

	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.OnStart(ctx)
	adv.SpawnSubscriptionHandler(ctx)
	require.NoError(t, adv.OnTick(ctx))
	failpoint.Enable(fp, "pause")
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		require.NoError(t, adv.OnTick(ctx))
	}()
	adv.OnStop()
	failpoint.Disable(fp)

	cp := c.advanceCheckpoints()
	c.flushAll()
	<-ch
	adv.WithCheckpoints(func(vsf *spans.ValueSortedFull) {
		// Advancer will manually poll the checkpoint...
		require.Equal(t, vsf.MinValue(), cp)
	})
}

// TestRemoveTaskAndFlush tests the bug has been described in #50839.
func TestRemoveTaskAndFlush(t *testing.T) {
	logutil.OverrideLevelForTest(t, zapcore.DebugLevel)
	ctx := context.Background()
	c := createFakeCluster(t, 4, true)
	installSubscribeSupport(c)
	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)
	adv.SpawnSubscriptionHandler(ctx)
	require.NoError(t, adv.OnTick(ctx))
	require.NoError(t, failpoint.Enable("github.com/pingcap/tidb/br/pkg/streamhelper/subscription-handler-loop", "pause"))
	c.flushAll()
	env.unregisterTask()
	require.Eventually(t, func() bool {
		return !adv.HasTask()
	}, 10*time.Second, 100*time.Millisecond)
	require.NoError(t, failpoint.Disable("github.com/pingcap/tidb/br/pkg/streamhelper/subscription-handler-loop"))
	require.Eventually(t, func() bool {
		return !adv.HasSubscriptions()
	}, 10*time.Second, 100*time.Millisecond)
}

func TestEnableCheckPointLimit(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := newTestEnv(c, t)
	rngs := env.ranges
	if len(rngs) == 0 {
		rngs = []kv.KeyRange{{}}
	}
	env.task = streamhelper.TaskEvent{
		Type: streamhelper.EventAdd,
		Name: "whole",
		Info: &backup.StreamBackupTaskInfo{
			Name:    "whole",
			StartTs: oracle.GoTimeToTS(oracle.GetTimeFromTS(0).Add(1 * time.Minute)),
		},
		Ranges: rngs,
	}
	log.Info("Start Time:", zap.Uint64("StartTs", env.task.Info.StartTs))

	adv := streamhelper.NewCommandCheckpointAdvancer(env)
	adv.UpdateCheckPointLagLimit(time.Minute)
	c.advanceClusterTimeBy(1 * time.Minute)
	c.advanceCheckpointBy(1 * time.Minute)
	adv.StartTaskListener(ctx)
	for range 5 {
		c.advanceClusterTimeBy(30 * time.Second)
		c.advanceCheckpointBy(20 * time.Second)
		require.NoError(t, adv.OnTick(ctx))
	}
}

func TestOwnerChangeCheckPointLagged(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := newTestEnv(c, t)
	rngs := env.ranges
	if len(rngs) == 0 {
		rngs = []kv.KeyRange{{}}
	}
	env.task = streamhelper.TaskEvent{
		Type: streamhelper.EventAdd,
		Name: "whole",
		Info: &backup.StreamBackupTaskInfo{
			Name:    "whole",
			StartTs: oracle.GoTimeToTS(oracle.GetTimeFromTS(0).Add(1 * time.Minute)),
		},
		Ranges: rngs,
	}

	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.UpdateCheckPointLagLimit(time.Minute)
	ctx1, cancel1 := context.WithCancel(context.Background())
	adv.OnStart(ctx1)
	adv.OnBecomeOwner(ctx1)
	log.Info("advancer1 become owner")
	require.NoError(t, adv.OnTick(ctx1))

	// another advancer but never advance checkpoint before
	adv2 := streamhelper.NewCheckpointAdvancer(env)
	adv2.UpdateCheckPointLagLimit(time.Minute)
	ctx2, cancel2 := context.WithCancel(context.Background())
	adv2.OnStart(ctx2)

	for range 5 {
		c.advanceClusterTimeBy(2 * time.Minute)
		c.advanceCheckpointBy(2 * time.Minute)
		require.NoError(t, adv.OnTick(ctx1))
	}
	c.advanceClusterTimeBy(2 * time.Minute)
	require.ErrorContains(t, adv.OnTick(ctx1), "lagged too large")

	// resume task to make next tick normally
	c.advanceCheckpointBy(2 * time.Minute)
	env.ResumeTask(ctx)

	// stop advancer1, and advancer2 should take over
	cancel1()
	log.Info("advancer1 owner canceled, and advancer2 become owner")
	adv2.OnBecomeOwner(ctx2)
	require.NoError(t, adv2.OnTick(ctx2))

	// advancer2 should take over and tick normally
	for range 10 {
		c.advanceClusterTimeBy(2 * time.Minute)
		c.advanceCheckpointBy(2 * time.Minute)
		require.NoError(t, adv2.OnTick(ctx2))
	}
	c.advanceClusterTimeBy(2 * time.Minute)
	require.ErrorContains(t, adv2.OnTick(ctx2), "lagged too large")
	// stop advancer2, and advancer1 should take over
	c.advanceCheckpointBy(2 * time.Minute)
	env.ResumeTask(ctx)
	cancel2()
	log.Info("advancer2 owner canceled, and advancer1 become owner")

	adv.OnBecomeOwner(ctx)
	// advancer1 should take over and tick normally when come back
	require.NoError(t, adv.OnTick(ctx))
}

func TestCheckPointLagged(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := newTestEnv(c, t)
	rngs := env.ranges
	if len(rngs) == 0 {
		rngs = []kv.KeyRange{{}}
	}
	env.task = streamhelper.TaskEvent{
		Type: streamhelper.EventAdd,
		Name: "whole",
		Info: &backup.StreamBackupTaskInfo{
			Name:    "whole",
			StartTs: oracle.GoTimeToTS(oracle.GetTimeFromTS(0).Add(1 * time.Minute)),
		},
		Ranges: rngs,
	}

	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.UpdateCheckPointLagLimit(time.Minute)
	adv.StartTaskListener(ctx)
	c.advanceClusterTimeBy(2 * time.Minute)
	// if global ts is not advanced, the checkpoint will not be lagged
	c.advanceCheckpointBy(2 * time.Minute)
	require.NoError(t, adv.OnTick(ctx))
	c.advanceClusterTimeBy(3 * time.Minute)
	require.ErrorContains(t, adv.OnTick(ctx), "lagged too large")
	// after some times, the isPaused will be set and ticks are skipped
	require.Eventually(t, func() bool {
		return assert.NoError(t, adv.OnTick(ctx))
	}, 5*time.Second, 100*time.Millisecond)
}

// If the paused task are manually resumed, it should run normally
func TestCheckPointResume(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.UpdateCheckPointLagLimit(time.Minute)
	adv.StartTaskListener(ctx)
	c.advanceClusterTimeBy(1 * time.Minute)
	// if global ts is not advanced, the checkpoint will not be lagged
	c.advanceCheckpointBy(1 * time.Minute)
	require.NoError(t, adv.OnTick(ctx))
	c.advanceClusterTimeBy(2 * time.Minute)
	require.ErrorContains(t, adv.OnTick(ctx), "lagged too large")
	require.Eventually(t, func() bool {
		return assert.NoError(t, adv.OnTick(ctx))
	}, 5*time.Second, 100*time.Millisecond)
	//now the checkpoint issue is fixed and resumed
	c.advanceCheckpointBy(1 * time.Minute)
	env.ResumeTask(ctx)
	require.Eventually(t, func() bool {
		return assert.NoError(t, adv.OnTick(ctx))
	}, 5*time.Second, 100*time.Millisecond)
	//with time passed, the checkpoint will exceed the limit again
	c.advanceClusterTimeBy(2 * time.Minute)
	require.ErrorContains(t, adv.OnTick(ctx), "lagged too large")
}

func TestUnregisterAfterPause(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.UpdateCheckPointLagLimit(time.Minute)
	adv.StartTaskListener(ctx)

	// wait for the task to be added
	require.Eventually(t, func() bool {
		return adv.HasTask()
	}, 5*time.Second, 100*time.Millisecond)

	// task is should be paused when global checkpoint is laggeod
	// even the global checkpoint is equal to task start ts(not advanced all the time)
	c.advanceClusterTimeBy(1 * time.Minute)
	require.NoError(t, adv.OnTick(ctx))
	env.PauseTask(ctx, "whole")
	c.advanceClusterTimeBy(1 * time.Minute)
	require.Error(t, adv.OnTick(ctx), "checkpoint is lagged")
	env.unregisterTask()
	env.putTask()

	// wait for the task to be added
	require.Eventually(t, func() bool {
		return adv.HasTask()
	}, 5*time.Second, 100*time.Millisecond)

	require.Error(t, adv.OnTick(ctx), "checkpoint is lagged")

	env.unregisterTask()
	// wait for the task to be deleted
	require.Eventually(t, func() bool {
		return !adv.HasTask()
	}, 5*time.Second, 100*time.Millisecond)

	// reset
	c.advanceClusterTimeBy(-1 * time.Minute)
	require.NoError(t, adv.OnTick(ctx))
	env.PauseTask(ctx, "whole")
	c.advanceClusterTimeBy(1 * time.Minute)
	env.unregisterTask()
	env.putTask()
	// wait for the task to be add
	require.Eventually(t, func() bool {
		return adv.HasTask()
	}, 5*time.Second, 100*time.Millisecond)

	require.Error(t, adv.OnTick(ctx), "checkpoint is lagged")
}

// If the start ts is *NOT* lagged, even both the cluster and pd are lagged, the task should run normally.
func TestAddTaskWithLongRunTask0(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := newTestEnv(c, t)
	rngs := env.ranges
	if len(rngs) == 0 {
		rngs = []kv.KeyRange{{}}
	}
	env.task = streamhelper.TaskEvent{
		Type: streamhelper.EventAdd,
		Name: "whole",
		Info: &backup.StreamBackupTaskInfo{
			Name:    "whole",
			StartTs: oracle.GoTimeToTS(oracle.GetTimeFromTS(0).Add(2 * time.Minute)),
		},
		Ranges: rngs,
	}

	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.UpdateCheckPointLagLimit(time.Minute)
	c.advanceClusterTimeBy(3 * time.Minute)
	c.advanceCheckpointBy(1 * time.Minute)
	env.advanceCheckpointBy(1 * time.Minute)
	env.mockPDConnectionError()
	adv.StartTaskListener(ctx)
	// Try update checkpoint
	require.NoError(t, adv.OnTick(ctx))
	// Verify no err raised
	require.NoError(t, adv.OnTick(ctx))
}

// If the start ts is lagged, as long as cluster has advanced, the task should run normally.
func TestAddTaskWithLongRunTask1(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := newTestEnv(c, t)
	rngs := env.ranges
	if len(rngs) == 0 {
		rngs = []kv.KeyRange{{}}
	}
	env.task = streamhelper.TaskEvent{
		Type: streamhelper.EventAdd,
		Name: "whole",
		Info: &backup.StreamBackupTaskInfo{
			Name:    "whole",
			StartTs: oracle.GoTimeToTS(oracle.GetTimeFromTS(0).Add(1 * time.Minute)),
		},
		Ranges: rngs,
	}

	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.UpdateCheckPointLagLimit(time.Minute)
	c.advanceClusterTimeBy(3 * time.Minute)
	c.advanceCheckpointBy(2 * time.Minute)
	env.advanceCheckpointBy(1 * time.Minute)
	adv.StartTaskListener(ctx)
	// Try update checkpoint
	require.NoError(t, adv.OnTick(ctx))
	// Verify no err raised
	require.NoError(t, adv.OnTick(ctx))
}

// If the start ts is lagged, as long as pd stored the advanced checkpoint, the task should run normally.
// Also, temporary connection error won't affect the task.
func TestAddTaskWithLongRunTask2(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := newTestEnv(c, t)
	rngs := env.ranges
	if len(rngs) == 0 {
		rngs = []kv.KeyRange{{}}
	}
	env.task = streamhelper.TaskEvent{
		Type: streamhelper.EventAdd,
		Name: "whole",
		Info: &backup.StreamBackupTaskInfo{
			Name:    "whole",
			StartTs: oracle.GoTimeToTS(oracle.GetTimeFromTS(0).Add(1 * time.Minute)),
		},
		Ranges: rngs,
	}

	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.UpdateCheckPointLagLimit(time.Minute)
	adv.StartTaskListener(ctx)
	c.advanceClusterTimeBy(3 * time.Minute)
	c.advanceCheckpointBy(1 * time.Minute)
	env.advanceCheckpointBy(2 * time.Minute)
	env.mockPDConnectionError()
	// if cannot connect to pd, the checkpoint will be rolled back
	// because at this point. the global ts is 2 minutes
	// and the local checkpoint ts is 1 minute
	require.Error(t, adv.OnTick(ctx), "checkpoint rollback")

	// only when local checkpoint > global ts, the next tick will be normal
	c.advanceCheckpointBy(12 * time.Minute)
	// Verify no err raised
	require.NoError(t, adv.OnTick(ctx))
}

// If the start ts, pd, and cluster checkpoint are all lagged, the task should pause.
func TestAddTaskWithLongRunTask3(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := newTestEnv(c, t)
	rngs := env.ranges
	if len(rngs) == 0 {
		rngs = []kv.KeyRange{{}}
	}
	env.task = streamhelper.TaskEvent{
		Type: streamhelper.EventAdd,
		Name: "whole",
		Info: &backup.StreamBackupTaskInfo{
			Name:    "whole",
			StartTs: oracle.GoTimeToTS(oracle.GetTimeFromTS(0).Add(1 * time.Minute)),
		},
		Ranges: rngs,
	}

	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.UpdateCheckPointLagLimit(time.Minute)
	// advance cluster time to 4 minutes, and checkpoint to 1 minutes
	// if start ts equals to checkpoint, the task will not be paused
	adv.StartTaskListener(ctx)
	c.advanceClusterTimeBy(2 * time.Minute)
	c.advanceCheckpointBy(1 * time.Minute)
	env.advanceCheckpointBy(1 * time.Minute)
	require.NoError(t, adv.OnTick(ctx))

	c.advanceClusterTimeBy(2 * time.Minute)
	c.advanceCheckpointBy(1 * time.Minute)
	env.advanceCheckpointBy(1 * time.Minute)
	// Try update checkpoint
	require.ErrorContains(t, adv.OnTick(ctx), "lagged too large")
	// Verify no err raised after paused
	require.Eventually(t, func() bool {
		err := adv.OnTick(ctx)
		return err == nil
	}, 5*time.Second, 300*time.Millisecond)
}

func TestOwnershipLost(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	c.splitAndScatter(manyRegions(0, 10240)...)
	installSubscribeSupport(c)
	ctx, cancel := context.WithCancel(context.Background())
	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.OnStart(ctx)
	adv.OnBecomeOwner(ctx)
	require.NoError(t, adv.OnTick(ctx))
	c.advanceCheckpoints()
	c.flushAll()
	failpoint.Enable("github.com/pingcap/tidb/br/pkg/streamhelper/subscription.listenOver.aboutToSend", "pause")
	failpoint.Enable("github.com/pingcap/tidb/br/pkg/streamhelper/FlushSubscriber.Clear.timeoutMs", "return(500)")
	defer func() {
		require.NoError(t, failpoint.Disable("github.com/pingcap/tidb/br/pkg/streamhelper/FlushSubscriber.Clear.timeoutMs"))
	}()
	wg := new(sync.WaitGroup)
	wg.Add(adv.TEST_registerCallbackForSubscriptions(wg.Done))
	cancel()
	failpoint.Disable("github.com/pingcap/tidb/br/pkg/streamhelper/subscription.listenOver.aboutToSend")
	wg.Wait()
}

func TestSubscriptionPanic(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	c.splitAndScatter(manyRegions(0, 20)...)
	installSubscribeSupport(c)
	ctx, cancel := context.WithCancel(context.Background())
	env := newTestEnv(c, t)
	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.OnStart(ctx)
	adv.OnBecomeOwner(ctx)
	wg := new(sync.WaitGroup)
	wg.Add(adv.TEST_registerCallbackForSubscriptions(wg.Done))

	require.NoError(t, adv.OnTick(ctx))
	failpoint.Enable("github.com/pingcap/tidb/br/pkg/streamhelper/subscription.listenOver.aboutToSend", "5*panic")
	defer func() {
		require.NoError(t, failpoint.Disable("github.com/pingcap/tidb/br/pkg/streamhelper/subscription.listenOver.aboutToSend"))
	}()
	ckpt := c.advanceCheckpoints()
	c.flushAll()
	cnt := 0
	for {
		require.NoError(t, adv.OnTick(ctx))
		cnt++
		if env.checkpoint >= ckpt {
			break
		}
		if cnt > 100 {
			t.Fatalf("After 100 times, the progress cannot be advanced.")
		}
	}
	cancel()
	wg.Wait()
}

func TestGCCheckpoint(t *testing.T) {
	c := createFakeCluster(t, 4, false)
	defer func() {
		fmt.Println(c)
	}()
	c.splitAndScatter("01", "02", "022", "023", "033", "04", "043")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := newTestEnv(c, t)
	rngs := env.ranges
	if len(rngs) == 0 {
		rngs = []kv.KeyRange{{}}
	}
	env.task = streamhelper.TaskEvent{
		Type: streamhelper.EventAdd,
		Name: "whole",
		Info: &backup.StreamBackupTaskInfo{
			Name:    "whole",
			StartTs: oracle.GoTimeToTS(oracle.GetTimeFromTS(0)),
		},
		Ranges: rngs,
	}
	log.Info("Start Time:", zap.Uint64("StartTs", env.task.Info.StartTs))

	adv := streamhelper.NewCheckpointAdvancer(env)
	adv.StartTaskListener(ctx)
	c.advanceClusterTimeBy(1 * time.Minute)
	c.advanceCheckpointBy(1 * time.Minute)
	env.PauseTask(ctx, "whole")
	c.serviceGCSafePoint = oracle.GoTimeToTS(oracle.GetTimeFromTS(0).Add(2 * time.Minute))
	env.ResumeTask(ctx)
	require.ErrorContains(t, adv.OnTick(ctx), "greater than the target")
}

func TestRedactBackend(t *testing.T) {
	info := new(backup.StreamBackupTaskInfo)
	info.Name = "test"
	info.Storage = &backup.StorageBackend{
		Backend: &backup.StorageBackend_S3{
			S3: &backup.S3{
				Endpoint:        "http://",
				Bucket:          "test",
				Prefix:          "test",
				AccessKey:       "12abCD!@#[]{}?/\\",
				SecretAccessKey: "12abCD!@#[]{}?/\\",
			},
		},
	}

	redacted := redact.TaskInfoRedacted{Info: info}
	require.Equal(t, redacted.String(), "storage:<s3:<endpoint:\"http://\" bucket:\"test\" prefix:\"test\" access_key:\"[REDACTED]\" secret_access_key:\"[REDACTED]\" sse_kms_key_id:\"[REDACTED]\" > > name:\"test\" ")
	require.Equal(t, info.String(), "storage:<s3:<endpoint:\"http://\" bucket:\"test\" prefix:\"test\" access_key:\"12abCD!@#[]{}?/\\\\\" secret_access_key:\"12abCD!@#[]{}?/\\\\\" > > name:\"test\" ")

	info.Storage = &backup.StorageBackend{
		Backend: &backup.StorageBackend_Gcs{
			Gcs: &backup.GCS{
				Endpoint:        "http://",
				Bucket:          "test",
				Prefix:          "test",
				CredentialsBlob: "12abCD!@#[]{}?/\\",
			},
		},
	}
	redacted = redact.TaskInfoRedacted{Info: info}
	require.Equal(t, redacted.String(), "storage:<gcs:<endpoint:\"http://\" bucket:\"test\" prefix:\"test\" credentials_blob:\"[REDACTED]\" > > name:\"test\" ")
	require.Equal(t, info.String(), "storage:<gcs:<endpoint:\"http://\" bucket:\"test\" prefix:\"test\" credentials_blob:\"12abCD!@#[]{}?/\\\\\" > > name:\"test\" ")

	info.Storage = &backup.StorageBackend{
		Backend: &backup.StorageBackend_AzureBlobStorage{
			AzureBlobStorage: &backup.AzureBlobStorage{
				Endpoint:  "http://",
				Bucket:    "test",
				Prefix:    "test",
				SharedKey: "12abCD!@#[]{}?/\\",
				AccessSig: "12abCD!@#[]{}?/\\",
				EncryptionKey: &backup.AzureCustomerKey{
					EncryptionKey:       "12abCD!@#[]{}?/\\",
					EncryptionKeySha256: "12abCD!@#[]{}?/\\",
				},
			},
		},
	}
	redacted = redact.TaskInfoRedacted{Info: info}
	require.Equal(t, redacted.String(), "storage:<azure_blob_storage:<endpoint:\"http://\" bucket:\"test\" prefix:\"test\" shared_key:\"[REDACTED]\" access_sig:\"[REDACTED]\" encryption_key:<encryption_key:\"[REDACTED]\" > > > name:\"test\" ")
	require.Equal(t, info.String(), "storage:<azure_blob_storage:<endpoint:\"http://\" bucket:\"test\" prefix:\"test\" shared_key:\"12abCD!@#[]{}?/\\\\\" access_sig:\"12abCD!@#[]{}?/\\\\\" encryption_key:<encryption_key:\"12abCD!@#[]{}?/\\\\\" encryption_key_sha256:\"12abCD!@#[]{}?/\\\\\" > > > name:\"test\" ")
}
