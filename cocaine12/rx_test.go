package cocaine12

import (
	"context"
	"log"
)

func Example_ApplicationClient() {
	ctx := context.Background()
	s, err := NewService(ctx, "log", nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create channle to communicate with method "ping"
	channel, err := s.Call(ctx, "enqueue", "ping")
	if err != nil {
		log.Fatal(err)
	}

	// `enqueue` accepts stream, so send the chunk of data
	if err = channel.Call(ctx, "write", "AAAAAA"); err != nil {
		log.Fatal(err)
	}
	defer channel.Call(ctx, "close")

	// receive the answer from the application
	answer, err := channel.Get(ctx)
	if err != nil {
		log.Fatal(err)
	}

	var res struct {
		Res string
	}

	// Unpack to an anonymous struct
	if err := answer.Extract(&res); err != nil {
		log.Fatal(err)
	}

	log.Printf("%s", res.Res)
}
