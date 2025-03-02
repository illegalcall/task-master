import { NextResponse, type NextRequest } from "next/server"

export async function middleware(request: NextRequest) {
  // Allow access to login page and API routes
  if (request.nextUrl.pathname.startsWith("/api/")) {
    return NextResponse.next()
  }

  if (request.nextUrl.pathname === "/login") {
    const token = request.cookies.get("token")
    if (token) {
      return NextResponse.redirect(new URL("/", request.url))
    }
    return NextResponse.next()
  }

  const token = request.cookies.get("token")

  // If no token is present, redirect to login
  if (!token) {
    return NextResponse.redirect(new URL("/login", request.url))
  }

  try {
    // Verify token by making a request to /api/jobs
    const response = await fetch(`${process.env.API_URL}/api/jobs`, {
      headers: {
        Authorization: `Bearer ${token.value}`,
      },
    })

    // If unauthorized, redirect to login
    if (response.status === 401) {
      return NextResponse.redirect(new URL("/login", request.url))
    }

    // If authorized, allow access
    return NextResponse.next()
  } catch (error) {
    // In case of any error, redirect to login
    return NextResponse.redirect(new URL("/login", request.url))
  }
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
