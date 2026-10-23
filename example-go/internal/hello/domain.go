package hello

type Greeting struct {
	Message string `json:"message"`
}

func NewGreeting() Greeting {
	return Greeting{
		Message: "Hola, Amigos",
	}
}
