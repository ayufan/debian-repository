package deb

type PackageSlice []*Package

func (a PackageSlice) Len() int {
	return len(a)
}

func (a PackageSlice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a PackageSlice) Less(i, j int) bool {
	if a[i].Name() < a[j].Name() {
		return true
	} else if a[i].Name() > a[j].Name() {
		return false
	}

	if a[i].Version() < a[j].Version() {
		return true
	} else if a[i].Version() > a[j].Version() {
		return false
	}

	if a[i].Architecture() < a[j].Architecture() {
		return true
	} else if a[i].Architecture() > a[j].Architecture() {
		return false
	}

	return false
}
