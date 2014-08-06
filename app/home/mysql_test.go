package home

import (
    "testing"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)

// ATTACH LEFTS AND RIGHTS TO TREE: It should be able to assign lefts and rights to a tree
func TestAttachLeftsAndRights(t *testing.T) {
    beforeEach("mysql")

    // Row 1 node 1
    expectedOut := &treeNode{Parent: nil, Children: nil, Node: Condition{Text: "AND", Type: "logic", Operator: "AND"}, Left: 1, Right: 24}

    // Row 2 node 1
    outChild1 := treeNode{Parent: nil, Children: nil, Node: Condition{Text: "OR", Type: "logic", Operator: "OR"}, Left: 2, Right: 17}
    // Row 2 node 2
    outChild2 := treeNode{Parent: nil, Children: nil, Node: Condition{Text: "OR", Type: "logic", Operator: "OR"}, Left: 18, Right: 23}
    expectedOut.Children = []*treeNode{&outChild1, &outChild2}

    // Row 3 node 1
    outChild3 := treeNode{Parent: &outChild1, Children: nil, Node: Condition{Text: "AND", Type: "logic", Operator: "AND"}, Left: 3, Right: 14}
    // Row 3 node 2
    outChild4 := treeNode{Parent: &outChild1, Children: nil, Node: Condition{Text: "age eq 1", Type: "equality", Field: "age", Operator: "eq", Value: "1"}, Left: 15, Right: 16}
    outChild1.Children = []*treeNode{&outChild3, &outChild4}

    // Row 3 node 3
    outChild5 := treeNode{Parent: &outChild2, Children: nil, Node: Condition{Text: "age eq 2", Type: "equality", Field: "age", Operator: "eq", Value: "2"}, Left: 19, Right: 20}
    // Row 3 node 4
    outChild6 := treeNode{Parent: &outChild2, Children: nil, Node: Condition{Text: "age eq 3", Type: "equality", Field: "age", Operator: "eq", Value: "3"}, Left: 21, Right: 22}
    outChild2.Children = []*treeNode{&outChild5, &outChild6}

    // Row 4 nodes 1-5
    outChild7 := treeNode{Parent: &outChild3, Children: nil, Node: Condition{Text: "age eq 4", Type: "equality", Field: "age", Operator: "eq", Value: "4"}, Left: 4, Right: 5}
    outChild8 := treeNode{Parent: &outChild3, Children: nil, Node: Condition{Text: "age eq 5", Type: "equality", Field: "age", Operator: "eq", Value: "5"}, Left: 6, Right: 7}
    outChild9 := treeNode{Parent: &outChild3, Children: nil, Node: Condition{Text: "age eq 6", Type: "equality", Field: "age", Operator: "eq", Value: "6"}, Left: 8, Right: 9}
    outChild10 := treeNode{Parent: &outChild3, Children: nil, Node: Condition{Text: "age eq 7", Type: "equality", Field: "age", Operator: "eq", Value: "7"}, Left: 10, Right: 11}
    outChild11 := treeNode{Parent: &outChild3, Children: nil, Node: Condition{Text: "age eq 8", Type: "equality", Field: "age", Operator: "eq", Value: "8"}, Left: 12, Right: 13}
    outChild3.Children = []*treeNode{&outChild7, &outChild8, &outChild9, &outChild10, &outChild11}

    testingTreeRoot.attachLeftsAndRights()

    if !testingTreeRoot.matches(expectedOut) {
        t.Errorf("%v.attachLeftsAndRights - got %v, want %v", testingTreeRoot, testingTreeRoot.print(), expectedOut.print())
    }
}

// CONVERT TREE TO MYSQL: It should be able to convert a tree to mysql input values
func TestToMysql(t *testing.T) {
    beforeEach("mysql")

    equalityReturned, logicReturned := testingTreeRoot.toMysql()

    if equalityReturned != testingMysqlEqualityInput {
        t.Errorf("%v.toMysql() equalityReturned - got %v, want %v", testingTreeRoot, equalityReturned, testingMysqlEqualityInput)
    }

    if logicReturned != testingMysqlLogicInput {
        t.Errorf("%v.toMysql() logicReturned - got %v, want %v", testingTreeRoot, logicReturned, testingMysqlLogicInput)
    }
}

func TestUpdateDatabase(t *testing.T) {
    var Field, Operator, Value string
    var Left, Right int
    var equalityRowsReturned []equalitySqlRow
    var logicRowsReturned []logicSqlRow

    db, _ := sql.Open("mysql", "root:@/")
    defer db.Close()

    updateDatabase(testingTreeRoot.toMysql())

    // Get equality sql rows
    rows, _ := db.Query("SELECT field, operator, value, lt, rt FROM logictree.equality")
    defer rows.Close()

    for rows.Next() {
        rows.Scan(&Field, &Operator, &Value, &Left, &Right)
        equalityRowsReturned = append(equalityRowsReturned, equalitySqlRow{Field: Field, Operator: Operator, Value: Value, Left: Left, Right: Right})
    }

    if !equalitySqlMatchesArray(equalityRowsReturned, testingMysqlEqualityOutput) {
        t.Errorf("updateDatabase(%v) equalityReturned - got %v, want %v", testingTreeRoot, equalityRowsReturned, testingMysqlEqualityOutput)
    }

    // Get logic sql rows
    rows, _ = db.Query("SELECT operator, lt, rt FROM logictree.logic")
    defer rows.Close()

    for rows.Next() {
        rows.Scan(&Operator, &Left, &Right)
        logicRowsReturned = append(logicRowsReturned, logicSqlRow{Operator: Operator, Left: Left, Right: Right})
    }

    if !logicSqlMatchesArray(logicRowsReturned, testingMysqlLogicOutput) {
        t.Errorf("updateDatabase(%v) equalityReturned - got %v, want %v", testingTreeRoot, logicRowsReturned, testingMysqlLogicOutput)
    }
}
