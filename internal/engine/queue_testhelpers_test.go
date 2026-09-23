package engine

func (q *CommandQueue) Dequeue() (QueuedCommand, bool) {
	command, _, ok := q.DequeueGen()
	return command, ok
}
