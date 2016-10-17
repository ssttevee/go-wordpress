package wordpress

func dedupe(ids []int64) (deduped []int64, idMap map[int64][]int) {
	idMap = make(map[int64][]int)
	for i, id := range ids {
		if _, ok := idMap[id]; !ok {
			deduped = append(deduped, id)
		}

		idMap[id] = append(idMap[id], i)
	}

	return
}
