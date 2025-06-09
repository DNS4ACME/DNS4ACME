package lang_test

import (
	"encoding/json"
	"fmt"
	"github.com/dns4acme/dns4acme/lang"
	"log"
)

func ExampleLinkedList_Iterator() {
	ll := &lang.LinkedList[int]{
		Item: 1,
		Next: &lang.LinkedList[int]{
			Item: 2,
			Next: &lang.LinkedList[int]{
				Item: 3,
			},
		},
	}
	for item := range ll.Iterator {
		fmt.Println(item)
	}
	//Output: 1
	//2
	//3
}

func ExampleLinkedList_Iterator2() {
	ll := &lang.LinkedList[int]{
		Item: 1,
		Next: &lang.LinkedList[int]{
			Item: 2,
			Next: &lang.LinkedList[int]{
				Item: 3,
			},
		},
	}
	for i, item := range ll.Iterator2 {
		fmt.Printf("%d: %d\n", i, item)
	}
	//Output: 0: 1
	//1: 2
	//2: 3
}

func ExampleLinkedList_UnmarshalJSON() {
	data := []byte(`[1,2,3]`)
	var result *lang.LinkedList[int]
	if err := json.Unmarshal(data, &result); err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Get(0))
	//Output: 1
}
