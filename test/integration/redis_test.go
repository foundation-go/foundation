//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	f "github.com/foundation-go/foundation"
	cablecourier "github.com/foundation-go/foundation/cable/courier"
	ferrpb "github.com/foundation-go/foundation/errors/proto"
	fjobs "github.com/foundation-go/foundation/jobs"
	fredis "github.com/foundation-go/foundation/redis"
)

func TestRedisComponentLifecycle(t *testing.T) {
	component := fredis.NewComponent(
		fredis.WithURL(redisURL),
		fredis.WithLogger(testLogger()),
	)

	require.NoError(t, component.Start())
	require.NotNil(t, component.Connection)

	assert.NoError(t, component.Health())
	assert.NoError(t, component.HealthContext(context.Background()))

	require.NoError(t, component.Stop())

	// A closed client has to fail the health check, or a torn-down service
	// stays in the load balancer.
	assert.Error(t, component.Health())
}

func TestRedisComponentFailsOnAnUnreachableServer(t *testing.T) {
	// Reserved as invalid by RFC 6890.
	component := fredis.NewComponent(
		fredis.WithURL("redis://192.0.2.1:6379"),
		fredis.WithLogger(testLogger()),
	)

	assert.Error(t, component.Start(), "Start pings, so an unreachable server must fail there")
}

// BuildRedisPool used to reassemble `host:port` by hand, which silently dropped
// the database number in the URL. Every service that pointed at
// `redis://host:6379/3` was quietly writing to database 0 instead — and sharing
// a keyspace it believed it had to itself.
func TestBuildRedisPoolHonoursTheDatabaseNumber(t *testing.T) {
	const database = 3

	key := unique("dbcheck")

	pool, err := f.BuildRedisPool(redisURL+"/"+strconv.Itoa(database), 2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })

	conn := pool.Get()
	_, err = conn.Do("SET", key, "written-to-db-3")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	// Read it back with an independent client on database 3...
	onThree := goredis.NewClient(&goredis.Options{Addr: redisAddr(t), DB: database})
	t.Cleanup(func() { _ = onThree.Close() })

	value, err := onThree.Get(context.Background(), key).Result()
	require.NoError(t, err, "the key should be in database %d", database)
	assert.Equal(t, "written-to-db-3", value)

	// ...and confirm it is not in database 0, which is where it used to land.
	onZero := goredis.NewClient(&goredis.Options{Addr: redisAddr(t), DB: 0})
	t.Cleanup(func() { _ = onZero.Close() })

	_, err = onZero.Get(context.Background(), key).Result()
	assert.ErrorIs(t, err, goredis.Nil, "the key must not be in database 0")
}

// Plenty of things accept a TCP connection without speaking Redis — a proxy, a
// mesh sidecar, the wrong port. A probe that only dialled called those healthy
// and left the failure for the first real command, somewhere far from the
// configuration that caused it.
//
// A local listener that accepts and hangs up is used rather than an unroutable
// address: on a machine behind a VPN or a captive network, "unroutable" ones are
// often answered by something.
func TestBuildRedisPoolRejectsAServerThatIsNotRedis(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}

			_ = conn.Close()
		}
	}()

	_, err = f.BuildRedisPool("redis://"+listener.Addr().String(), 2)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not answer as Redis")
}

// A password in the URL must not come back in the error.
func TestBuildRedisPoolDoesNotLeakThePasswordOnFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}

			_ = conn.Close()
		}
	}()

	_, err = f.BuildRedisPool("redis://user:hunter2@"+listener.Addr().String(), 2)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "hunter2")
}

func TestBuildRedisPoolRejectsAnEmptyURL(t *testing.T) {
	_, err := f.BuildRedisPool("   ", 2)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "REDIS_URL")
}

// The pool must not hand out connections a server-side timeout has already
// dropped; TestOnBorrow is what verifies them.
func TestBuildRedisPoolRecoversFromAServerSideClose(t *testing.T) {
	pool, err := f.BuildRedisPool(redisURL, 2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })

	key := unique("borrow")

	conn := pool.Get()
	_, err = conn.Do("SET", key, "one")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	// Kill every client the server knows about, including the pooled one.
	admin := goredis.NewClient(&goredis.Options{Addr: redisAddr(t)})
	t.Cleanup(func() { _ = admin.Close() })
	require.NoError(t, admin.Do(context.Background(), "CLIENT", "KILL", "TYPE", "normal", "SKIPME", "yes").Err())

	// The pool should notice and reconnect rather than returning a dead socket.
	eventually(t, 5*time.Second, "the pool to recover from a server-side close", func() bool {
		c := pool.Get()
		defer c.Close() //nolint:errcheck // returning it to the pool

		_, doErr := c.Do("GET", key)

		return doErr == nil
	})
}

// The jobs enqueuer used to dereference a nil Enqueuer in Health, and its pool
// is built by BuildRedisPool.
func TestJobsEnqueuerAgainstRealRedis(t *testing.T) {
	namespace := unique("jobs")

	pool, err := f.BuildRedisPool(redisURL, 3)
	require.NoError(t, err)

	component := fjobs.NewComponent(
		fjobs.WithRedisPool(pool),
		fjobs.WithNamespace(namespace),
		fjobs.WithLogger(testLogger()),
	)

	require.NoError(t, component.Start())
	t.Cleanup(func() { _ = component.Stop() })

	assert.NoError(t, component.Health())
	assert.NoError(t, component.HealthContext(context.Background()))

	job, err := component.Enqueuer.Enqueue("send_email", map[string]interface{}{
		f.CorrelationIDArg: "corr-1",
		"to":               "someone@example.com",
	})
	require.NoError(t, err)
	require.NotNil(t, job)

	// gocraft/work keeps its queues under `<namespace>:jobs:<name>`.
	client := goredis.NewClient(&goredis.Options{Addr: redisAddr(t)})
	t.Cleanup(func() { _ = client.Close() })

	length, err := client.LLen(context.Background(), namespace+":jobs:send_email").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), length, "the job should be queued in Redis")
}

// The AnyCable envelope is assembled by hand; a subscriber has to be able to
// read it back.
func TestCableCourierBroadcastReachesASubscriber(t *testing.T) {
	channel := unique("anycable")

	client := goredis.NewClient(&goredis.Options{Addr: redisAddr(t)})
	t.Cleanup(func() { _ = client.Close() })

	subscription := client.Subscribe(context.Background(), channel)
	t.Cleanup(func() { _ = subscription.Close() })

	// Wait for the subscription to be live, or the publish races it.
	_, err := subscription.Receive(context.Background())
	require.NoError(t, err)

	courier := cablecourier.NewClient(client, channel)

	require.NoError(t, courier.BroadcastMessage(
		context.Background(),
		"foundation.errors.NotFoundError",
		&ferrpb.NotFoundError{Kind: "chat", Id: "1"},
		"user:42",
		"corr-1",
	))

	select {
	case message := <-subscription.Channel():
		var envelope cablecourier.Event
		require.NoError(t, json.Unmarshal([]byte(message.Payload), &envelope))
		assert.Equal(t, "user:42", envelope.Stream)

		// `data` holds the JSON *text* of the payload, still quoted.
		var payloadJSON string
		require.NoError(t, json.Unmarshal([]byte(envelope.Data), &payloadJSON))

		var payload cablecourier.EventData
		require.NoError(t, json.Unmarshal([]byte(payloadJSON), &payload))

		assert.Equal(t, "foundation.errors.NotFoundError", payload.Event)
		assert.Equal(t, "corr-1", payload.CorrelationID)
		assert.Equal(t, "chat", payload.Data["kind"])
	case <-time.After(5 * time.Second):
		t.Fatal("the broadcast never reached the subscriber")
	}
}

// A cancelled context must abort the publish rather than blocking the courier.
func TestCableCourierBroadcastHonoursTheContext(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: redisAddr(t)})
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := cablecourier.NewClient(client, unique("anycable")).BroadcastMessage(
		ctx, "x.Y", &ferrpb.NotFoundError{}, "user:1", "corr",
	)

	assert.Error(t, err)
}

func TestServiceGetRedisReturnsALiveClient(t *testing.T) {
	t.Setenv("REDIS_URL", redisURL)
	t.Setenv("METRICS_ENABLED", "false")

	svc := &f.Service{Name: "test", Config: f.NewConfig(), Logger: testLogger()}

	require.NoError(t, svc.StartComponents())
	t.Cleanup(svc.StopComponents)

	client, err := svc.TryGetRedis()
	require.NoError(t, err)

	key := unique("service")
	require.NoError(t, client.Set(context.Background(), key, "value", time.Minute).Err())

	value, err := client.Get(context.Background(), key).Result()
	require.NoError(t, err)
	assert.Equal(t, "value", value)
}

// redisAddr returns the container's host:port, without the scheme.
func redisAddr(t *testing.T) string {
	t.Helper()

	options, err := goredis.ParseURL(redisURL)
	require.NoError(t, err)

	return options.Addr
}
