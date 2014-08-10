package home

import (
    "fmt"
    "encoding/json"
    "net/http"
    "html/template"
    "errors"
    "database/sql"

    _ "github.com/go-sql-driver/mysql"
    "git-go-logictree/app/common"
)

type Condition struct {
    Text, Type, Field, Operator, Value string
}

type treeNode struct {
    Parent *treeNode
    Children []*treeNode
    Node Condition
    Left, Right int
}

type conditionSqlRow struct {
    Field, Operator, Value, Type string
    Left, Right int
}

func GetHomePage(rw http.ResponseWriter, req *http.Request) {
    type Page struct {
        Title string
        Conditions []Condition
    }
    
    p := Page{
        Title: "home",
        Conditions: getConditions(),
    }

    common.Templates = template.Must(template.ParseFiles("templates/home/home.html", common.LayoutPath))
    err := common.Templates.ExecuteTemplate(rw, "base", p)
    common.CheckError(err, 2)
}

func UpdateConditions(rw http.ResponseWriter, req *http.Request) {
    conditions := req.FormValue("conditions");

    parsedConditions, _ := parseJSON(conditions);
    fmt.Println(parsedConditions)
    treeRoot, err := unserializeFormattedTree(parsedConditions)

    equalityStr, logicStr, err := treeRoot.toMysql()
    if err != nil {
        fmt.Println(err)
        return
    }

    updateDatabase(equalityStr, logicStr)

    GetHomePage(rw, req)
}

func Truncate(rw http.ResponseWriter, req *http.Request) {
    _, err := common.DB.Query("TRUNCATE TABLE logictree.equality")
    common.CheckError(err, 2)

    _, err = common.DB.Query("TRUNCATE TABLE logictree.logic")
    common.CheckError(err, 2)

    GetHomePage(rw, req)
}

func getConditions() []Condition {
    conditions := make([]Condition, 0)

    rows, err := common.DB.Query("SELECT field, operator, value FROM logictree.equality")
    common.CheckError(err, 2)

    var field, operator, value string

    i := 0

    for rows.Next() {
        rows.Scan(&field, &operator, &value)
        common.CheckError(err, 2)

        if i != 0 {
            conditions = append(conditions, Condition{
                Text: "AND",
                Operator: "AND",
                Type: "logic",
            })
        }

        conditions = append(conditions, Condition{
            Text: fmt.Sprintf("%s %s %s", field, operator, value),
            Type: "equality",
            Field: field,
            Operator: operator,
            Value: value,
        })

        i++
    }

    return conditions
}

func parseJSON(conditionsString string) ([]Condition, error) {
    var conditionsSlice []Condition
    
    err := json.Unmarshal([]byte(conditionsString), &conditionsSlice)
    if err != nil {
        return nil, err
    }
    
    return conditionsSlice, nil
}

/** Treat conditions like a queue. Rules:
 * If you reach a (, pop the condition, drop down a depth and assign results to root's children
 * If you reach a ), pop the condition, pop back up a depth with the root
 * If you reach a logical condition, pop the condition, assign it as the root's node
 * If you reach an equality condition, pop the condition, assign it as one of the children of the root
 * At the end of the loop, return the root's first child (since we have parans around all conditions we're going to be one level too deep)
**/
func unserializeFormattedTree(conditions []Condition) (*treeNode, error) {
    var root treeNode
    var emptyNode, condition Condition

    key := 0

    for key < len(conditions) {
        // Pop the front item from the slice
        condition = conditions[0]
        conditions = append(conditions[:key], conditions[key+1:]...)

        switch condition.Type {
        case "scope":
            if condition.Operator == "(" {
                children, _ := unserializeFormattedTree(conditions)

                if len(root.Children) == 0 {
                    root.Children = []*treeNode{children}
                } else {
                    root.Children = append(root.Children, children)
                }
            }

            if condition.Operator == ")" {
                if root.Node == emptyNode {
                    return root.Children[0], nil
                } else {
                    return &root, nil
                }
            }
        case "logic":
            root.Node = condition
        case "equality":
            root.Children = append(root.Children, &treeNode{Parent: &root, Node: condition})
        }
    }

    if len(root.Children) > 0 && !root.Node.matches(emptyNode) {
        return &root, nil
    }

    return root.Children[0], nil
}

/**
 * Assumption: data should be ordered by LEFT
 * Recursive steps:
 * If current node is a branch:
 *      Create node with it
 *      Pop item from conditions
 *      Add children recursively
 * If node is a leaf:
 *      Create node with it
 *      Pop item from conditions
 * Return node
**/
func unserializeRawTree(conditions []conditionSqlRow) *treeNode {
    return nil
}

func serializeTree(node *treeNode) ([]Condition, error) {
    if node.Children == nil || len(node.Children) == 0 {
        // Has no children - should be equality

        if node.Node.Type != "equality" {
            return nil, errors.New("ERROR: This tree has a logic condition as a leaf. Quitting.")
        }

        return []Condition{node.Node}, nil
    } else {
        // Has children - should be logic

        if node.Node.Type != "logic" {
            return nil, errors.New("ERROR: This tree has an equality condition as a branch. Quitting.")
        }
    }

    linearConditions := []Condition{Condition{Text: "(", Type: "scope", Operator: "("}}

    for key, child := range node.Children {
        if key != 0 {
            linearConditions = append(linearConditions, node.Node)
        }

        serializedChild, err := serializeTree(child)

        if err != nil {
            return nil, err
        }

        linearConditions = append(linearConditions, serializedChild...)
    }

    linearConditions = append(linearConditions, Condition{Text: ")", Type: "scope", Operator: ")"})

    return linearConditions, nil
}

func (t *treeNode) attachLeftsAndRights() {
    indexStart := 0
    t.attachLeftsAndRightsRecursively(&indexStart)
}

func (t *treeNode) attachLeftsAndRightsRecursively(index *int) {
    *index++
    t.Left = *index

    for _, child := range t.Children {
        child.attachLeftsAndRightsRecursively(index)
    }

    *index++
    t.Right = *index
}

func (t *treeNode) toMysql() (equalityStr, logicStr string, err error) {
    t.attachLeftsAndRights()

    equalityStr, logicStr = t.toMysqlRecursively()

    if len(equalityStr) == 0 {
        return "", "", errors.New(fmt.Sprintf("Error: equality string was empty, which was unexpected. EqualityStr: %s :: LogicStr: %s", equalityStr, logicStr))
    }

    if len(logicStr) == 0 {
        return "", "", errors.New(fmt.Sprintf("Error: logic string was empty, which was unexpected. EqualityStr: %s :: LogicStr: %s", equalityStr, logicStr))
    }

    equalityStr = equalityStr[:(len(equalityStr)-1)]
    logicStr = logicStr[:(len(logicStr)-1)]

    return equalityStr, logicStr, nil
}

func (t *treeNode) toMysqlRecursively() (equalityStr, logicStr string) {
    var equalityTemp, logicTemp string
    for _, child := range t.Children {
        equalityTemp, logicTemp = child.toMysqlRecursively()
        equalityStr += equalityTemp
        logicStr += logicTemp
    }

    switch (t.Node.Type) {
    case "equality":
        equalityStr += fmt.Sprintf("('%s', '%s', '%s', 'equality', %d, %d),", t.Node.Field, t.Node.Operator, t.Node.Value, t.Left, t.Right)
    case "logic":
        logicStr += fmt.Sprintf("('%s', 'logic', %d, %d),", t.Node.Operator, t.Left, t.Right)
    }

    return equalityStr, logicStr
}

func (t *treeNode) print() string {
    var s string

    for _, child := range t.Children {
        s += child.print()
    }

    return s + " :: " + fmt.Sprintf("%v, %d, %d", t.Node, t.Left, t.Right)
}

func simplifyConditions(conditions []Condition) string {
    var t string

    for k, c := range conditions {
        if k != 0 {
            t += " "
        }

        t += c.Text
    }

    return t
}

func (t *treeNode) getChildrenConditions() []Condition {
    var children []Condition

    for _, child := range t.Children {
        children = append(children, child.Node)
    }

    return children
}

func updateDatabase(equalityStr, logicStr string) {
    db, _ := sql.Open("mysql", "root:@/")
    defer db.Close()

    _, err := db.Query("TRUNCATE TABLE logictree.conditions")
    common.CheckError(err, 2)

    _, err = db.Query("INSERT INTO logictree.conditions (field, operator, value, type, lt, rt) VALUES "+equalityStr)
    common.CheckError(err, 2)
    _, err = db.Query("INSERT INTO logictree.conditions (operator, type, lt, rt) VALUES "+logicStr)
    common.CheckError(err, 2)
}

func (conditionA Condition) matches(conditionB Condition) bool {
    if conditionA.Text != conditionB.Text {
        return false
    }

    if conditionA.Type != conditionB.Type {
        return false
    }

    if conditionA.Field != conditionB.Field {
        return false
    }

    if conditionA.Operator != conditionB.Operator {
        return false
    }

    if conditionA.Value != conditionB.Value {
        return false
    }

    return true
}






