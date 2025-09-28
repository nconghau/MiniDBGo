package main

func matchQuery(doc map[string]interface{}, q map[string]interface{}) bool {
	for k, v := range q {
		// simple equal match
		if doc[k] != v {
			return false
		}
	}
	return true
}
