// Package logs implements `kubectl ome logs`: component-aware log streaming
// for the pods behind an InferenceService.
package logs

import (
	"bufio"
	"io"
	"sync"
)

type namedStream struct {
	Prefix string
	Reader io.ReadCloser
}

// multiplex copies every stream to out line-by-line, prefixing each line,
// serialized by a mutex so lines never interleave mid-line. Blocks until all
// streams hit EOF (or error); reader close is guaranteed.
func multiplex(streams []namedStream, out io.Writer) error {
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs = make([]error, len(streams))
	)
	for i, s := range streams {
		wg.Add(1)
		go func(i int, s namedStream) {
			defer wg.Done()
			defer s.Reader.Close()
			scanner := bufio.NewScanner(s.Reader)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				mu.Lock()
				_, werr := io.WriteString(out, s.Prefix+scanner.Text()+"\n")
				mu.Unlock()
				if werr != nil {
					errs[i] = werr
					return
				}
			}
			errs[i] = scanner.Err()
		}(i, s)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
