package pathing

import (
	"container/heap"
	"github.com/google/uuid"
	"github.com/rubinda/GoWorld"
	"math"
)

var (
	// Adjacent directions without the center point
	directions8 = [8]GoWorld.Location{
		{-1, -1},
		{0, -1},
		{1, -1},
		{1, 0},
		{1, 1},
		{0, 1},
		{-1, 1},
		{-1, 0},
	}
	// Adjacent directions without the center point
	directions4 = [4]GoWorld.Location{
		{0, -1},
		{1, 0},
		{0, 1},
		{-1, 0},
	}
	worldWidth = 0
)

type AStar struct {
	World GoWorld.World
}

type Brownian struct {
}

// PathNeighborCost returns the cost to the tile from 1 tile away based on terrain surface type
func (n *aStarNode) PathNeighborCost(to *aStarNode, w GoWorld.World) float64 {
	// TODO handle error
	surfaceName, _ := w.GetSurfaceNameAt(GoWorld.Location{X: to.X, Y: to.Y})
	// Cost to this spot is based on surface type:
	switch surfaceName {
	case "Grassland":
		// Grass is easiest to walk on
		return 1.0
	case "Gravel":
		// Gravel is a bit harder to walk on than grass
		return 1.5
	case "Forest":
		// Trees slow you down
		return 2.0
	case "Mountain":
		// Slopes are hardest to cross
		return 2.5
	default:
		// Unknown surface, assume it is harder to cross than others
		return 3.0
	}
}

// PathEstimatedCost tries to predict the distance between the nodes
// We use a Euclidean distance, but without the square root (faster computation, values are just larger)
func (n *aStarNode) PathEstimatedCost(to *aStarNode) float64 {
	return math.Pow(float64(to.X)-float64(n.X), 2) + math.Pow(float64(to.Y)-float64(n.Y), 2)
	// Euclidean distance
	//return math.Abs(float64(ps.X - toSpot.X)) + math.Abs(float64(ps.Y - toSpot.Y))
}

// Return all neighbours we can move to
func (n *aStarNode) PathNeighbors(w GoWorld.World) []*aStarNode {
	neighbours := []*aStarNode{}
	// Check the neighbouring spots in 8 directions
	for _, offset := range directions8 {
		newX := n.X + offset.X
		newY := n.Y + offset.Y
		newLocation := GoWorld.Location{X: newX, Y: newY}
		occupyingBeing, _ := w.GetBeingAt(newLocation)
		habitable, _ := w.IsHabitable(newLocation)
		// Check if the neighbouring spot is blocked (surface not passable or being on it)
		if habitable && occupyingBeing == uuid.Nil {
			// Surface is without a being and can be walked on, add to neighbours
			neighbours = append(neighbours, &aStarNode{X: newX, Y: newY})
		}
	}
	return neighbours
}

// New initializes the pathfinder
func NewPathfinder(world GoWorld.World) GoWorld.Pathfinder {
	a := &AStar{
		World: world,
	}
	// Store the world size for node ID generation
	worldWidth, _ = a.World.GetSize()
	return a
}

// GetPath returns a list of locations (moves) towards the desired location
func (a *AStar) GetPath(from GoWorld.Location, to GoWorld.Location) []GoWorld.Location {
	//fmt.Println("Path from ", from, "to ", to, " distance: ", a.World.Distance(from, to))
	// Check if both spots are valid to walk on
	toHab, _ := a.World.IsHabitable(to)
	toOccupied, _ := a.World.GetBeingAt(to)
	if !toHab || toOccupied != uuid.Nil {
		// TODO return error and handle it there?
		// Location to which we want to move is not inhabitable, return an empty path
		return []GoWorld.Location{}
	}

	// Create node out of location for path searching
	fromSpot := aStarNode{
		X: from.X,
		Y: from.Y,
	}
	toSpot := aStarNode{
		X: to.X,
		Y: to.Y,
	}

	// Find a path using the A* algorithm
	path, _, found := astar(fromSpot, toSpot, a.World)
	//fmt.Println("path -> locations array")
	if !found {
		// TODO return error and handle it there?
		//n, _ := a.World.GetSurfaceNameAt(to)
		//fn, _ := a.World.GetSurfaceNameAt(from)
		////fmt.Println("No path found from ", from, "to ", to)
		return []GoWorld.Location{}
	}

	// Convert the nodes back to locations for use in other GoWorld packages
	locations := make([]GoWorld.Location, len(path))
	j := 0
	for i := len(path) - 1; i >= 0; i-- {
		locations[j] = GoWorld.Location{
			X: path[i].X,
			Y: path[i].Y,
		}
		j++
	}
	//fmt.Println("Path search return")
	return locations
}

// astar calculates a short path and the distance between the two nodes
// If no path is found, found will be false
// PATH IS RETURNED IN REVERSE ORDER, FIRST NODE IS TO, LAST IS FROM
func astar(from, to aStarNode, w GoWorld.World) (path []*aStarNode, distance float64, found bool) {
	// The open and closed lists from A*
	// The open list is a priority queue for performance reasons
	openList := &aStarQueue{indexOf: make(map[int64]int)}
	heap.Init(openList)
	// The closed list should be a set, but for simplicity it is a map where keys work as the set
	closedList := make(map[int64]bool)

	// Calculate the node IDs
	from.calculateID()
	to.calculateID()

	// Add the source node and start exploring paths
	heap.Push(openList, from)
	for {
		if openList.Len() == 0 {
			// There's no astar, return found false.
			return
		}
		// Select next node and add it to the closed list (it has been visited, do not check in the future)
		currentNode := heap.Pop(openList).(aStarNode)
		closedList[currentNode.id] = true

		// Check if we reached the goal
		if currentNode.id == to.id {
			// Reconstruct the path from current node
			foundPath := []*aStarNode{}
			ancestor := &currentNode
			for ancestor != nil {
				foundPath = append(foundPath, ancestor)
				ancestor = ancestor.parent
			}
			// Return path, distance, and that we found a path
			return foundPath, currentNode.gScore, true
		}
		// Explore every suitable neighbour of the current node

		for _, neighbour := range currentNode.PathNeighbors(w) {
			// Calculate the ID if it doesn't exist
			neighbour.calculateID()

			// If the neighbour is in the closed list (has been visited already), do not revisit him
			if _, ok := closedList[neighbour.id]; ok {
				continue
			}

			// Calculate statistics
			// G score ... the cost from source node to this one
			// H score ... the current heuristic of cost left till reaching sink node
			// F score ... G + H .. how long we think this path may be
			neighbourG := currentNode.gScore + currentNode.PathNeighborCost(neighbour, w)
			neighbourH := neighbour.PathEstimatedCost(&to)
			neighbourF := neighbourG + neighbourH

			// Check if the neighbour is already in the open list (nodes we plan to visit in the future)
			if existingNeighbour, ok := openList.node(neighbour.id); !ok {
				// Neighbour was not in open list, add a new entry to it
				heap.Push(openList, aStarNode{
					X:      neighbour.X,
					Y:      neighbour.Y,
					id:     neighbour.id,
					parent: &currentNode,
					gScore: neighbourG,
					fScore: neighbourF,
				})
			} else if neighbourG < existingNeighbour.gScore {
				// Neighbour is already in the open list, probably from a different path, but this path gives the node
				// a lower G score, so update that node in the list
				existingNeighbour.parent = &currentNode
				openList.update(existingNeighbour.id, neighbourG, neighbourF)
			}
		}
	}
}

// aStarNode represents a node in the A* searching algorithm
type aStarNode struct {
	X, Y   int        // The location of the spot in the world
	id     int64      // The unique identifier
	parent *aStarNode // The node that brought us to this node
	gScore float64    // The score from the source node to here
	fScore float64    // The heuristic estimation of the distance from source to sink on this path
}

// CalculateID sets and returns the node identifier
// Imagine raveling 2D array into 1D and the 1D index becomes the ID of the node
// The operation is idempotent
func (n *aStarNode) calculateID() int64 {
	id := int64(n.Y*worldWidth + n.X)
	n.id = id
	return id
}

// aStarQueue is an A* priority queue
type aStarQueue struct {
	indexOf map[int64]int
	nodes   []aStarNode
}

// GetNode returns the node with the ID from the queue
func (q *aStarQueue) getNode(id int64) aStarNode {
	return q.nodes[q.indexOf[id]]
}

// Less is a comparator function
func (q *aStarQueue) Less(i, j int) bool {
	return q.nodes[i].fScore < q.nodes[j].fScore
}

// Swap replaces the positions of the elements in the queue
func (q *aStarQueue) Swap(i, j int) {
	q.indexOf[q.nodes[i].id] = j
	q.indexOf[q.nodes[j].id] = i
	q.nodes[i], q.nodes[j] = q.nodes[j], q.nodes[i]
}

// Len return queue length
func (q *aStarQueue) Len() int {
	return len(q.nodes)
}

// Push adds an object to the end of the queue
func (q *aStarQueue) Push(x interface{}) {
	n := x.(aStarNode)
	q.indexOf[n.id] = len(q.nodes)
	q.nodes = append(q.nodes, n)
}

// Pop returns the last elemenet in the queue
func (q *aStarQueue) Pop() interface{} {
	n := q.nodes[len(q.nodes)-1]
	q.nodes = q.nodes[:len(q.nodes)-1]
	delete(q.indexOf, n.id)
	return n
}

// Update updates the G-Score and F-Score values of a node and rebuilds the heap
func (q *aStarQueue) update(id int64, g, f float64) {
	i, ok := q.indexOf[id]
	if !ok {
		return
	}
	q.nodes[i].gScore = g
	q.nodes[i].fScore = f
	heap.Fix(q, i)
}

// Node check if an aStarNode with given ID exists and returns it
func (q *aStarQueue) node(id int64) (aStarNode, bool) {
	loc, ok := q.indexOf[id]
	if ok {
		return q.nodes[loc], true
	}
	return aStarNode{}, false
}
