package lib

import (
	"log"
	"os"
	"sync"

	"github.com/OwnLocal/go-freshbooks"
)

type ProjectHours struct {
	Project     freshbooks.Project
	BudgetHours float64
	WorkedHours float64
}

func FreshBooksOrganizationName() string {
	return os.Getenv("FBKS_ORG_NAME")
}

func FreshBooksEncryptedAPIKey() string {
	return os.Getenv("FBKS_API_KEY")
}

func FreshBooksSlackVerificationToken() string {
	return os.Getenv("FBKS_SLACK_VERIFICATION_TOKEN")
}

func AuthenticateFreshbooksApi(freshbooksOrgName string, apiKey string) *freshbooks.Api {
	return freshbooks.NewApi(freshbooksOrgName, apiKey)
}

func HourBundlesForActiveProjects(api *freshbooks.Api) []ProjectHours {
	projects, _, err := api.ListProjects(freshbooks.Request{})
	if err != nil {
		log.Fatal("Failed to fetch list of projects: %s", err)
	}

	return extractHourBundlesFromProjects(api, *projects)
}

func extractHourBundlesFromProjects(api *freshbooks.Api, projects []freshbooks.Project) []ProjectHours {
	c := make(chan ProjectHours, len(projects))
	defer close(c)
	wg := new(sync.WaitGroup)
	activeProjectCount := 0

	for _, project := range projects {
		if project.HourBudget > 0 {
			wg.Add(1)
			activeProjectCount += 1
			go hourBundleForProject(api, project, c, wg)
		}
	}

	wg.Wait()

	return drainProjectHoursChannel(c)
}

func drainProjectHoursChannel(c <-chan ProjectHours) []ProjectHours {
	hours := make([]ProjectHours, 0)

ForLoop:
	for {
		select {
		case hoursForProject := <-c:
			hours = append(hours, hoursForProject)
		default:
			break ForLoop
		}
	}

	return hours
}

func hourBundleForProject(api *freshbooks.Api, project freshbooks.Project, c chan<- ProjectHours, wg *sync.WaitGroup) {
	defer wg.Done()

	timeEntries := timeEntriesForProject(api, project)

	c <- extractHourBundleFromProjectTimeEntries(project, timeEntries)
}

func extractHourBundleFromProjectTimeEntries(project freshbooks.Project, timeEntries []freshbooks.TimeEntry) ProjectHours {
	totalHoursWorked := 0.0
	for _, entry := range timeEntries {
		totalHoursWorked += entry.Hours
	}

	return ProjectHours{project, project.HourBudget, totalHoursWorked}
}

func timeEntriesForProject(api *freshbooks.Api, project freshbooks.Project) []freshbooks.TimeEntry {
	perPage := 100
	allTimeEntries := make([]freshbooks.TimeEntry, 0)

	timeEntries, pagination, err := api.ListTimeEntries(freshbooks.Request{ProjectId: project.ProjectId, PerPage: perPage})
	if err != nil {
		log.Fatal("Failed to fetch list of time entries: %s", err)
	}
	allTimeEntries = append(allTimeEntries, *timeEntries...)

	if pagination.Pages > 1 {
		timeEntriesWaitGroup := new(sync.WaitGroup)
		c := make(chan []freshbooks.TimeEntry, pagination.Pages)

		for i := 2; i <= pagination.Pages; i++ {
			timeEntriesWaitGroup.Add(1)
			go timeEntriesForProjectPage(api, project, i, perPage, c, timeEntriesWaitGroup)
		}

		timeEntriesWaitGroup.Wait()
		allTimeEntries = append(allTimeEntries, drainTimeEntriesChannel(c)...)
	}

	return allTimeEntries
}

func timeEntriesForProjectPage(api *freshbooks.Api, project freshbooks.Project, page int, perPage int, c chan<- []freshbooks.TimeEntry, wg *sync.WaitGroup) {
	defer wg.Done()

	timeEntries, _, err := api.ListTimeEntries(freshbooks.Request{ProjectId: project.ProjectId, PerPage: perPage, Page: page})

	if err != nil {
		log.Fatal("Failed to fetch time entries for project", project.ProjectId)
	}

	c <- *timeEntries
}

func drainTimeEntriesChannel(c <-chan []freshbooks.TimeEntry) []freshbooks.TimeEntry {
	timeEntries := make([]freshbooks.TimeEntry, 0)

ForLoop:
	for {
		select {
		case timeEntriesForProject := <-c:
			timeEntries = append(timeEntries, timeEntriesForProject...)
		default:
			break ForLoop
		}
	}

	return timeEntries
}
