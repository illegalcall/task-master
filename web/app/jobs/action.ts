import { fetchWithAuth } from "@/utils/auth"

export async function fetchJobs() {
  try {
    const res = await fetchWithAuth("http://localhost:8080/api/jobs")
    console.log(res, "inside action")
    const data = await res.jobs
    return data
  } catch (error) {
    console.error("Error fetching jobs:", error)
    return []
  }
}
