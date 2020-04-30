package resolvers

import (
	"errors"
	"reflect"
)

func (this *ResolveRequest) RunAsync(resolver Resolution) Resolution {

	type resolution struct {
		result reflect.Value
		err    error
	}

	channel := make(chan *resolution, 1)
	r := resolution{
		result: reflect.Value{},
		err:    errors.New("Unknown"),
	}

	// Limit the number of concurrent go routines that we startup.
	*this.ExecutionContext.GetLimiter() <- 1
	go func() {

		// Setup some post processing
		defer func() {
			err := this.ExecutionContext.HandlePanic(this.SelectionPath())
			if err != nil {
				r.err = err
			}
			<-*this.ExecutionContext.GetLimiter()
			channel <- &r // we do this in defer since the resolver() could panic.
		}()

		//  Stash the results, then get sent back in the defer.
		r.result, r.err = resolver()
	}()

	// Return a resolver that waits for the async results
	return func() (reflect.Value, error) {
		r := <-channel
		return r.result, r.err
	}
}
