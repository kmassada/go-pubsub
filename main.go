package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/user"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var (
	// Set to true if you want this task to publish
	shouldPublish = os.Getenv("PUBLISH")
	// Set to true if you want this task to subscribe
	shouldSubscribe = os.Getenv("SUBSCRIBE")

	projectname = os.Getenv("PROJECT_ID")

	// How fast do you want to broadcast?
	broadcastRate = 150 * time.Millisecond

	topicName = "pubsub-repro"
	subName   = func() string {
		i := rand.Intn(9)
		if s := os.Getenv("SUB_SUFFIX"); s != "" {
			i, _ = strconv.Atoi(s)
		}
		return fmt.Sprintf("pubsub-repro-subscription-%d", i)
	}
)

// Subscription represents methods on a subscription object.
type Subscription struct {
	subscription *pubsub.Subscription
	topic        *pubsub.Topic
	context      context.Context
	cancelFunc   context.CancelFunc

	nCallbacks int32 // atomic
}

func (sub *Subscription) process() {
	fmt.Printf("Listening for messages...\n\n")
	for {
		err := sub.subscription.Receive(sub.context, func(ctx context.Context, msg *pubsub.Message) {
			atomic.AddInt32(&sub.nCallbacks, 1)
			defer atomic.AddInt32(&sub.nCallbacks, -1)
			fmt.Printf("Message received: %q\n", msg.ID)
			msg.Ack()
		})
		//retryable
		if status.Code(err) == codes.Unavailable {
	        fmt.Print("check for status codes error")
	        time.Sleep(10*time.Second)
	        continue
	    }
	    //non-retryable error
		if err != nil {
			fmt.Printf("process() Receive Error: %v\n", err)
			debug.PrintStack()
		}
		fmt.Printf("Bailing\n")
		sub.Stop()
	}
}

func (sub *Subscription) Stop() {
	fmt.Println("Invoking cancelFunc()")
	sub.cancelFunc()
}

func (sub *Subscription) logNCallbacks() {
	for {
		time.Sleep(5 * time.Second)
		fmt.Printf("nCallbacks = %d\n", atomic.LoadInt32(&sub.nCallbacks))
	}
}

func createOrGetTopic(ctx context.Context, client *pubsub.Client) (*pubsub.Topic, error) {
	// Attempt to create the Topic
	topic, err := client.CreateTopic(ctx, topicName)
	if err != nil && grpc.Code(err) != codes.AlreadyExists {
		fmt.Printf("createOrGetTopic() Error: %#v\n", err)
		return nil, err
	}
	if topic == nil {
		topic = client.Topic(topicName)
	}
	fmt.Printf("Using topic: %s\n", topic.ID())
	return topic, nil
}

func createOrGetSubscription(ctx context.Context, client *pubsub.Client, topic *pubsub.Topic) (*pubsub.Subscription, error) {
	name := subName()
	// Attempt to create the Subscription
	sub, err := client.CreateSubscription(ctx, name, pubsub.SubscriptionConfig{
		Topic: topic,
	})
	if err != nil && grpc.Code(err) != codes.AlreadyExists {
		fmt.Printf("createOrGetSubscription() Error: %v\n", err)
		return nil, err
	}
	if sub == nil {
		sub = client.Subscription(name)
	}
	fmt.Printf("Using subscription: %s\n", sub.ID())
	return sub, nil
}

func createSubscription(ctx context.Context, c *pubsub.Client) *Subscription {
	topic, err := createOrGetTopic(ctx, c)
	if err != nil {
		return nil
	}

	sub, err := createOrGetSubscription(ctx, c, topic)
	if err != nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Subscription{
		subscription: sub,
		topic:        topic,
		context:      ctx,
		cancelFunc:   cancel,
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	ctx := context.Background()

	client, err := newClient(ctx)
	if err != nil {
		fmt.Printf("Err: %v\n", err)
		return
	}
	if client == nil {
		fmt.Printf("Client is unexpectedly nil\n")
		return
	}

	res := createSubscription(ctx, client)
	go res.logNCallbacks()
	if shouldPublish == "true" {
		ticker := time.NewTicker(broadcastRate)
		go func(res *Subscription) {
			fmt.Println("Starting ticker by broadcasing a message!")
			res.publish(time.Now())
			for t := range ticker.C {
				res.publish(t)
			}
		}(res)
		defer ticker.Stop()
	}

	if shouldSubscribe == "true" {
		go res.process()
	}
	select {}
}

func (sub *Subscription) publish(t time.Time) {
	msg := make([]byte, 16)
	rand.Read(msg)
	res := sub.topic.Publish(sub.context, &pubsub.Message{
		Data: msg,
	})
	m, err := res.Get(sub.context)
	if err != nil {
		fmt.Printf("Unsuccessfully pushed message: %v\n", err)
		return
	}
	fmt.Printf("Broadcasting new message at %s (msg: %s).\n", t, m)
}

func newClient(ctx context.Context) (*pubsub.Client, error) {
	fp, err := fileloc()
	if err != nil {
		return nil, fmt.Errorf("could not get file: %v", err)
	}
	c, err := pubsub.NewClient(ctx, projectname, option.WithCredentialsFile(fp))
	if err != nil {
		return nil, err
	}
	return c, nil
}

func fileloc() (string, error) {
	if f := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); f != "" {
		return f, nil
	}
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("could not determine user: %v", err)
	}
	return fmt.Sprintf("%s/pubsub.json", u.HomeDir), nil
}
