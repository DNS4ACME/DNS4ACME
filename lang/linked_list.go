package lang

import (
	"encoding/json"
	"fmt"
)

// LinkedList is a wrapper that allows linking together items in a list.
type LinkedList[T any] struct {
	Item T
	Next *LinkedList[T]
}

// Len returns the length of the LinkedList. This function is expensive as it needs to walk all items in the list.
func (list *LinkedList[T]) Len() int {
	if list == nil {
		return 0
	}
	return list.Next.Len() + 1
}

// Iterator is a function that is compatible with iter.Seq. You can use this function to loop over the linked list
// with a single key.
func (list *LinkedList[T]) Iterator(yield func(T) bool) {
	if list == nil {
		return
	}
	if !yield(list.Item) {
		return
	}
	if list.Next != nil {
		list.Next.Iterator(yield)
	}
}

// Iterator2 is a function that is compatible with iter.Seq2. You can use this function to iterate over the linked
// list as you would over a slice with indexes preserved.
func (list *LinkedList[T]) Iterator2(yield func(i int, v T) bool) {
	list.iterator2(0, yield)
}

func (list *LinkedList[T]) iterator2(i int, yield func(i int, v T) bool) {
	if list == nil {
		return
	}
	if !yield(i, list.Item) {
		return
	}
	list.Next.iterator2(i+1, yield)
}

// Slice converts the linked list into a slice.
func (list *LinkedList[T]) Slice() []T {
	result := make([]T, list.Len())
	for i, item := range list.Iterator2 {
		result[i] = item
	}
	return result
}

// AnySlice converts the linked list into a slice of any.
func (list *LinkedList[T]) AnySlice() []any {
	result := make([]any, list.Len())
	for i, item := range list.Iterator2 {
		result[i] = item
	}
	return result
}

// MarshalJSON converts the linked list into a JSON array.
func (list *LinkedList[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(list.Slice())
}

// UnmarshalJSON converts a JSON array into a linked list.
func (list *LinkedList[T]) UnmarshalJSON(data []byte) error {
	if list == nil {
		var def T
		panic(fmt.Errorf("LinkedList[%T]: unexpected nil pointer", def))
	}
	var slice []T
	if err := json.Unmarshal(data, &slice); err != nil {
		return err
	}
	*list = *LinkedListFromSlice(slice)
	return nil
}

// Get returns an item from the linked list. If the get is out of bounds, this function panics. Use GetChecked
// to check if the get is in bounds.
func (list *LinkedList[T]) Get(i int) T {
	item, ok := list.get(i, 0)
	if !ok {
		panic(&boundsError{
			i, list.Len(),
		})
	}
	return item
}

func (list *LinkedList[T]) GetChecked(i int) (T, bool) {
	return list.get(i, 0)
}

func (list *LinkedList[T]) get(i int, visited int) (def T, ok bool) {
	if i < 0 {
		return def, false
	}
	if list == nil {
		return def, false
	}
	if i == 0 {
		return list.Item, true
	}
	return list.Next.get(i-1, visited+1)
}

// LinkedListFromSlice converts a slice into a LinkedList of the same type.
func LinkedListFromSlice[T any](slice []T) *LinkedList[T] {
	var result *LinkedList[T]
	for i := len(slice) - 1; i >= 0; i-- {
		result = &LinkedList[T]{
			Item: slice[i],
			Next: result,
		}
	}
	return result
}

type boundsError struct {
	index  int
	length int
}

func (e *boundsError) Error() string {
	return fmt.Sprintf("linked list index %d out of range (list has %d items)", e.index, e.length)
}
