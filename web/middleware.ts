import { NextResponse, type NextRequest } from "next/server"

export const verifyToken = async (token: string) => {
  try {
    // Use the /api/jobs endpoint which is protected and will validate the token
    const apiUrl = process.env.API_URL || "http://localhost:8080"
    const response = await fetch(`${apiUrl}/api/jobs`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      // Add cache: 'no-store' to prevent caching issues
      cache: "no-store",
    })

    // A 200 status indicates successful authentication
    if (response.ok) {
      return true
    }

    // Any other status (including 401 Unauthorized) means verification failed
    return false
  } catch (error) {
    console.error("Token verification error:", error)
    return false
  }
}

export async function middleware(request: NextRequest) {
  // Allow access to login page, sign-up page, API routes, and auth callback
  if (request.nextUrl.pathname.startsWith("/api/") || 
      request.nextUrl.pathname.startsWith("/auth/")) {
    return NextResponse.next()
  }

  // Public routes that don't require authentication
  if (request.nextUrl.pathname === "/login" || request.nextUrl.pathname === "/sign-up") {
    const token = request.cookies.get("token")
    if (token) {
      try {
        const isVerified = await verifyToken(token.value)
        if (isVerified) {
          return NextResponse.redirect(new URL("/", request.url))
        }
      } catch (error) {
        console.error("Error verifying token on public page:", error)
        // Continue to public page if verification fails
      }
    }
    return NextResponse.next()
  }

  const token = request.cookies.get("token")

  if (!token) {
    return NextResponse.redirect(new URL("/login", request.url))
  }

  try {
    const isVerified = await verifyToken(token.value)
    if (!isVerified) {
      return NextResponse.redirect(new URL("/login", request.url))
    }
  } catch (error) {
    console.error("Error in middleware token verification:", error)
    return NextResponse.redirect(new URL("/login", request.url))
  }

  return NextResponse.next()
}

// Configure which routes to run middleware on
export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * 1. /api routes
     * 2. /_next (Next.js internals)
     * 3. /_static (inside /public)
     * 4. /_vercel (Vercel internals)
     * 5. /favicon.ico, /sitemap.xml (static files)
     */
    "/((?!api|_next|_static|_vercel|favicon.ico|sitemap.xml).*)",
  ],
}
