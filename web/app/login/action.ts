export async function loginUser(username : any, password : any) {
    try {
      const response = await fetch("http://localhost:8080/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      if (!response.ok) {
        throw new Error(`Login failed with status ${response.status}`);
      }
      const data = await response.json();
      localStorage.setItem("accessToken", data.token);
      localStorage.setItem("tokenType", data.type);
      return data; 
    } catch (error) {
      console.error("Error logging in:", error);
      throw error; 
    }
}
  