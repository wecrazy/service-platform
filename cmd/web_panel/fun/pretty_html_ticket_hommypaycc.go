package fun

import "strings"

func OdooStageHTML(stage string) string {
	var htmlStage string

	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "new":
		htmlStage = `<span class="badge bg-label-info">New</span>`
	case "in progress":
		htmlStage = `<span class="badge bg-warning">In Progress</span>`
	case "solved":
		htmlStage = `<span class="badge bg-success">Solved</span>`
	case "cancelled", "canceled":
		htmlStage = `<span class="badge bg-label-danger">Cancelled</span>`
	case "on hold":
		htmlStage = `<span class="badge bg-secondary">On Hold</span>`
	default:
		htmlStage = `<span class="badge bg-primary">N/A</span>`
	}

	return htmlStage
}

func Priority3StarsHTML(priority string) string {
	var star string

	priority = strings.ToLower(strings.TrimSpace(priority))
	switch priority {
	case "0":
		star = `<i class="fal fa-star text-secondary"></i>
				<i class="fal fa-star text-secondary"></i>
				<i class="fal fa-star text-secondary"></i>`
	case "1":
		star = `<i class="fas fa-star text-secondary"></i>
				<i class="fal fa-star text-secondary"></i>
				<i class="fal fa-star text-secondary"></i>`
	case "2":
		star = `<i class="fas fa-star text-secondary"></i>
				<i class="fas fa-star text-secondary"></i>
				<i class="fal fa-star text-secondary"></i>`
	case "3":
		star = `<i class="fas fa-star text-secondary"></i>
				<i class="fas fa-star text-secondary"></i>
				<i class="fas fa-star text-secondary"></i>`
	default:
		star = `<i class="fal fa-star text-secondary"></i>
				<i class="fal fa-star text-secondary"></i>
				<i class="fal fa-star text-secondary"></i>`
	}

	return star
}
