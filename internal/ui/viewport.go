package ui

// viewportWindow calculates the start index for a scrolling viewport.
// itemCount is the total number of items, maxVisible is how many fit on screen,
// cursor is the currently selected index. Returns the start index to slice from.
func viewportWindow(itemCount, maxVisible, cursor int) int {
	if itemCount <= maxVisible {
		return 0
	}
	// Keep cursor visible with some context
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	// Don't scroll past the end
	if start > itemCount-maxVisible {
		start = itemCount - maxVisible
	}
	if start < 0 {
		start = 0
	}
	return start
}

// viewportWindowCentered calculates start and end indices for a centered
// scrolling viewport. The cursor is kept near the middle of the window.
// itemCount is the total number of items, maxVisible is how many fit on screen,
// cursor is the currently selected index.
func viewportWindowCentered(itemCount, maxVisible, cursor int) (start, end int) {
	if maxVisible < 1 {
		maxVisible = 1
	}
	start = cursor - maxVisible/2
	if start < 0 {
		start = 0
	}
	end = start + maxVisible
	if end > itemCount {
		end = itemCount
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}
	return
}
