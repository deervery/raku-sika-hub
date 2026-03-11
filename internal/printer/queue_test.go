package printer

import "testing"

func TestParseLPRequestID(t *testing.T) {
	got := parseLPRequestID("request id is Brother_QL-820NWB-14 (1 file(s))")
	if got != "Brother_QL-820NWB-14" {
		t.Fatalf("unexpected job id: %q", got)
	}
}

func TestParseLPStatJobs(t *testing.T) {
	jobs, err := parseLPStatJobs("Brother_QL-820NWB-13 rakusika 1024 Thu 12 Mar 2026 03:27:25 JST\n")
	if err != nil {
		t.Fatalf("parseLPStatJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("unexpected job count: %d", len(jobs))
	}
	if jobs[0].ID != "Brother_QL-820NWB-13" {
		t.Fatalf("unexpected job id: %q", jobs[0].ID)
	}
	if jobs[0].SizeBytes != 1024 {
		t.Fatalf("unexpected job size: %d", jobs[0].SizeBytes)
	}
	if jobs[0].Destination != "Brother_QL-820NWB" {
		t.Fatalf("unexpected destination: %q", jobs[0].Destination)
	}
}
