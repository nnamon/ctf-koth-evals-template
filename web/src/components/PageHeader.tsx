import { Link, NavLink } from "react-router-dom";
import { useTheme } from "../theme";
import { useAuth } from "../auth";

// PageHeader is a thin .top-row inside the .page wrapper — brand on the left,
// nav + theme + logout on the right. This matches the kit's document-style
// chrome (AESTHETIC.md: "breadcrumbs over nav bars"); the heavy palette
// topbar from the showcase is intentionally not used.
export function PageHeader() {
  const { theme, toggle } = useTheme();
  const { authenticated, logout } = useAuth();

  return (
    <header className="top-row">
      <Link className="brand" to="/">
        ctf-evals
      </Link>
      <nav className="page-nav">
        {authenticated && (
          <>
            <NavLink
              to="/"
              end
              className={({ isActive }: { isActive: boolean }) =>
                isActive ? "page-nav-link active" : "page-nav-link"
              }
            >
              suites
            </NavLink>
            <NavLink
              to="/submit"
              className={({ isActive }: { isActive: boolean }) =>
                isActive ? "page-nav-link active" : "page-nav-link"
              }
            >
              submit
            </NavLink>
            <NavLink
              to="/submissions"
              className={({ isActive }: { isActive: boolean }) =>
                isActive ? "page-nav-link active" : "page-nav-link"
              }
            >
              submissions
            </NavLink>
          </>
        )}
        <button type="button" className="page-nav-link" onClick={toggle}>
          {theme === "dark" ? "light" : "dark"}
        </button>
        {authenticated && (
          <button type="button" className="page-nav-link" onClick={logout}>
            logout
          </button>
        )}
      </nav>
    </header>
  );
}
