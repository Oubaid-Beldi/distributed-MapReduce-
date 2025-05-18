package mymapreduce

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"strings"
	// "projet/repartie/common"

)





// KeyValue represents a key-value pair for MapReduce
type KeyValue struct {
	Key   string
	Value string
}

// ihash computes a hash for partitioning keys to reduce tasks
func ihash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// mapF processes a document and returns key-value pairs (word counts)
func MapF(document string, content string) []KeyValue {
	counts := make(map[string]int)
	words := strings.Fields(content)
	for _, word := range words {
		word = strings.ToLower(strings.Trim(word, ".,!?:;\"'"))
		if word != "" {
			counts[word]++
		}
	}
	kvs := make([]KeyValue, 0, len(counts))
	for word, count := range counts {
		kvs = append(kvs, KeyValue{Key: word, Value: fmt.Sprintf("%d", count)})
	}
	return kvs
}

// reduceF aggregates values for a key (sums word occurrences)
func ReduceF(key string, values []string) string {
	count := 0
	for _, v := range values {
		var n int
		fmt.Sscanf(v, "%d", &n)
		count += n
	}
	return fmt.Sprintf("%d", count)
}

// doMap reads input file, applies mapF, and partitions output to intermediate files
func DoMap(jobName string, mapTaskNumber int, inFile string, nReduce int, mapF func(string, string) []KeyValue) {
	content, err := os.ReadFile(inFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doMap: failed to read %s: %v\n", inFile, err)
		return
	}
	kvs := mapF(inFile, string(content))
	for i := 0; i < nReduce; i++ {
		fileName := fmt.Sprintf("mr-%d-%d", mapTaskNumber, i)
		fmt.Printf("doMap: creating file %s\n", fileName)
		f, err := os.Create(fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "doMap: failed to create %s: %v\n", fileName, err)
			continue
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		for _, kv := range kvs {
			if int(ihash(kv.Key)%uint32(nReduce)) == i {
				err := enc.Encode(&kv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "doMap: failed to encode to %s: %v\n", fileName, err)
				}
			}
		}
	}
}

// doReduce reads intermediate files, applies reduceF, and writes final output
func DoReduce(jobName string, reduceTaskNumber int, nMap int, reduceF func(string, []string) string) {
	kvMap := make(map[string][]string)
	for i := 0; i < nMap; i++ {
		fileName := fmt.Sprintf("mr-%d-%d", i, reduceTaskNumber)
		fmt.Printf("doReduce: reading file %s\n", fileName)
		f, err := os.Open(fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "doReduce: failed to open %s: %v\n", fileName, err)
			continue
		}
		defer f.Close()
		dec := json.NewDecoder(f)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			kvMap[kv.Key] = append(kvMap[kv.Key], kv.Value)
		}
	}
	outputFile := fmt.Sprintf("mr-out-%d", reduceTaskNumber)
	fmt.Printf("doReduce: writing to %s\n", outputFile)
	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doReduce: failed to create %s: %v\n", outputFile, err)
		return
	}
	defer f.Close()
	keys := make([]string, 0, len(kvMap))
	for k := range kvMap {
		keys = append(keys, k)
	}
	for _, key := range keys {
		result := reduceF(key, kvMap[key])
		fmt.Fprintf(f, "%s %s\n", key, result)
	}
}