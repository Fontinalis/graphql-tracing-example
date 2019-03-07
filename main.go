package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"sourcegraph.com/sourcegraph/appdash"
	appdashtracer "sourcegraph.com/sourcegraph/appdash/opentracing"
	"sourcegraph.com/sourcegraph/appdash/traceapp"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"

	opentracing "github.com/opentracing/opentracing-go"
	jaeger "github.com/uber/jaeger-client-go"
	config "github.com/uber/jaeger-client-go/config"
)

type User struct {
	Name  string
	Email string
}

type Book struct {
	ID   string
	Name string
}

var UserType = graphql.NewObject(graphql.ObjectConfig{
	Name: "User",
	Fields: graphql.Fields{
		"name": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				time.Sleep(100 * time.Millisecond)
				return p.Source.(User).Name, nil
			},
		},
		"email": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				time.Sleep(900 * time.Millisecond)
				return p.Source.(User).Email, nil
			},
		},
	},
})

var BookType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Book",
	Fields: graphql.Fields{
		"id": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				time.Sleep(50 * time.Millisecond)
				return p.Source.(Book).ID, nil
			},
		},
		"name": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				time.Sleep(300 * time.Millisecond)
				return p.Source.(Book).Name, nil
			},
		},
	},
})

var QueryType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Query",
	Fields: graphql.Fields{
		"foo": &graphql.Field{
			Type: graphql.String,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				time.Sleep(300 * time.Millisecond)
				return "ok", nil
			},
		},
		"bar": &graphql.Field{
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				time.Sleep(200 * time.Millisecond)
				return 1, nil
			},
		},
		"user": &graphql.Field{
			Type: UserType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				u := User{Name: "gopher", Email: "gopher@go.com"}
				return u, nil
				/*
					type result struct {
						data interface{}
						err  error
					}
					ch := make(chan *result, 1)
					go func() {
						defer close(ch)
						time.Sleep(500 * time.Millisecond)
						ch <- &result{data: u, err: nil}
					}()
					return func() (interface{}, error) {
						r := <-ch
						return r.data, r.err
					}, nil
				*/
			},
		},
		"books": &graphql.Field{
			Type: graphql.NewList(BookType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				b1 := Book{ID: "103", Name: "The Go Programming Language"}
				b2 := Book{ID: "1034", Name: "Go in Practice"}
				books := []Book{b1, b2}
				return books, nil
				/*
					type result struct {
						data interface{}
						err  error
					}
					ch := make(chan *result, 1)
					go func() {
						defer close(ch)
						time.Sleep(800 * time.Millisecond)
						ch <- &result{data: books, err: nil}
					}()
					return func() (interface{}, error) {
						r := <-ch
						return r.data, r.err
					}, nil
				*/
			},
		},
	},
})

func initJaeger(service string) (opentracing.Tracer, io.Closer) {
	cfg := &config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans: true,
		},
	}
	tracer, closer, err := cfg.New(service, config.Logger(jaeger.StdLogger))
	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}
	return tracer, closer
}

func main() {
	tracer, closer := initJaeger("graphql")
	defer closer.Close()

	opentracing.SetGlobalTracer(tracer)
	//startAppdashServer()

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: QueryType,
	})
	schema.AddExtensions(&Tracer{})
	if err != nil {
		log.Fatal(err)
	}
	h := handler.New(&handler.Config{
		Schema:   &schema,
		Pretty:   true,
		GraphiQL: true,
	})
	http.HandleFunc("/graphql", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span, ctx := opentracing.StartSpanFromContext(r.Context(), "/graphql")
		h.ContextHandler(ctx, w, r)
		span.Finish()
	}))
	log.Printf("server running on :8080")
	http.ListenAndServe(":8080", nil)
}

func startAppdashServer() opentracing.Tracer {
	memStore := appdash.NewMemoryStore()
	store := &appdash.RecentStore{
		MinEvictAge: 5 * time.Minute,
		DeleteStore: memStore,
	}

	url, err := url.Parse("http://localhost:8700")
	if err != nil {
		log.Fatal(err)
	}
	tapp, err := traceapp.New(nil, url)
	if err != nil {
		log.Fatal(err)
	}
	tapp.Store = store
	tapp.Queryer = memStore

	go func() {
		log.Fatal(http.ListenAndServe(":8700", tapp))
	}()
	tapp.Store = store
	tapp.Queryer = memStore

	collector := appdash.NewLocalCollector(store)

	tracer := appdashtracer.NewTracer(collector)

	opentracing.SetGlobalTracer(tracer)

	log.Println("Appdash web UI running on HTTP :8700")
	return tracer
}
