package main

//  doubly linked list

const (
	LIST_HEAD = 1
	LIST_TAIL = 2
)

type Node struct {
	Val  *GObj
	next *Node
	pre  *Node
}

type ListType struct {
	EqualFunc func(i *GObj, j *GObj) bool //定义判断2个元素是否相等
}

type List struct {
	ListType
	head   *Node
	tail   *Node
	length int
}

func ListCreate(listType ListType) *List {
	return &List{
		ListType: listType,
		length:   0,
		head:     nil,
		tail:     nil,
	}
}

// Find if not found return nil
func (list *List) Find(val *GObj) *Node {
	p := list.head
	for p != nil {
		if list.EqualFunc(p.Val, val) {
			break
		}
		p = p.next
	}
	return p
}

// TailPush insert node at the tail
func (list *List) TailPush(val *GObj) {
	var n Node
	n.Val = val
	if list.head == nil {
		list.head = &n
		list.tail = &n
	} else {
		n.pre = list.tail
		list.tail.next = &n
		list.tail = list.tail.next
	}
	list.length += 1
}

// HeadPush insert node at the head
func (list *List) HeadPush(val *GObj) {
	var n Node
	n.Val = val
	if list.head == nil {
		list.head = &n
		list.tail = &n
	} else {
		n.next = list.head
		list.head.pre = &n
		list.head = &n
	}
	list.length += 1
}

func (list *List) DelNode(n *Node) {
	if n == nil {
		return
	}
	if list.Length() == 1 && list.head == n {
		list.head = nil
		list.tail = nil
		list.length = 0
		return
	}

	if list.head == n {
		list.head = n.next
		n.next.pre = nil
		n.next = nil
	} else if list.tail == n {
		list.tail = n.pre
		n.pre.next = nil
		n.pre = nil
	} else {
		n.pre.next = n.next
		n.next.pre = n.pre
		n.pre = nil
		n.next = nil
	}
	list.length -= 1
}

// Index Return the element at the specified zero-based index
// where 0 is the head, 1 is the element next to head and so on.
// Negative integers are used in order to count from the tail,
// -1 is the last element, -2 the penultimate and so on.
// If the index is out of range nil is returned
func (list *List) Index(index int64) *Node {
	var n *Node
	if index < 0 {
		index = -index - 1
		n = list.tail
		for index > 0 && n != nil {
			n = n.pre
		}
	} else {
		n = list.tail
		for index > 0 && n != nil {
			n = n.next
		}
	}
	return n
}

func (list *List) Delete(val *GObj) {
	list.DelNode(list.Find(val))
}

func (list *List) First() *Node {
	return list.head
}

func (list *List) Last() *Node {
	return list.tail
}

func (list *List) Length() int {
	return list.length
}

func (list *List) TypePush(obj *GObj, where int) {
	if where == LIST_HEAD {
		list.HeadPush(obj)
	} else if where == LIST_TAIL {
		list.TailPush(obj)
	}
}

func (list *List) TypePop(where int) *GObj {
	var ln *Node
	if where == LIST_HEAD {
		ln = list.First()
	} else if where == LIST_TAIL {
		ln = list.Last()
	}
	list.DelNode(ln)
	return ln.Val
}
