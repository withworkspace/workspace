package c

func use(things map[string]interface{ Run() }) {
	for _, t := range things {
		t.Run()
		//^ candidates 3 :Run "by name"
	}
	undefinedFunc()
	// <- unresolved
}
