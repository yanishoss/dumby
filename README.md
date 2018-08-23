# :ram: Dumby

> Dumby is a super lightweight and fast protocol

## :satellite: How to download ?   
```shell
$ go get -u github.com/yanishoss/dumby/...
```

## :electric_plug: How to use ?

```golang
/*

In this file, we are going to create a simple Dumby server.
It will print the client's message and reply back.


Go ahead, even a Dumb can do that!

*/

package main

import (
	"fmt"

	"github.com/yanishoss/dumby/protocol"
	"github.com/yanishoss/dumby/server"
)

const (
	// The address the server will listen to
	host = "localhost:8080"

	actionHello = iota + 2
)

func main() {
	s := server.New(&server.Config{
		MaxConnections: 5000,
	})

	s.AddHandlers(actionHello, func(trame *protocol.Trame, s chan<- *protocol.Trame) {
		// Print the message sent by the client
		fmt.Println("Someone says: ", string(trame.Payload))

		// Respond back with a message
		s <- protocol.New(trame.Session, actionHello, []byte("Hello my friend! Is Dumby the best protocol on this world ?"))
	})

	s.Listen(host)

	// So it's not difficult, right ?
}
```  
**You did it** :clap:

[See more example](./example "Examples")

## :running: Yet to come  

* A simple client
* A more robust server implementation
