import { Link, useLocation } from 'react-router-dom';
import WalletButton from './WalletButton';
import { GitHubLoginButton } from './GitHubLoginButton';
import { useAuth } from '../hooks/useAuth';

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { user, logout } = useAuth();

  const navLink = (path: string, label: string) => {
    const active = location.pathname === path;
    return (
      <Link
        to={path}
        className={`text-sm font-medium px-3 py-1.5 rounded-md transition-all duration-200 ${active
            ? 'bg-solana-purple/15 text-solana-purple'
            : 'text-gray-500 hover:text-gray-200 hover:bg-solana-card'
          }`}
      >
        {label}
      </Link>
    );
  };

  return (
    <div className="min-h-screen flex flex-col bg-solana-dark bg-vignette">
      <header className="border-b border-solana-border">
        <div className="max-w-6xl mx-auto px-6 py-3.5 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <Link to="/" className="flex items-center gap-2.5 group">
              <div className="w-8 h-8 rounded-full bg-gradient-to-br from-solana-purple to-solana-green flex items-center justify-center font-bold text-[11px] transition-transform duration-300 group-hover:scale-105">
                E
              </div>
              <span className="text-sm font-bold text-white hidden sm:inline tracking-tight">
                Enshor
              </span>
            </Link>

            <div className="h-5 w-px bg-solana-border hidden md:block" />

            <nav className="hidden md:flex items-center gap-1">
              {navLink('/', 'Campaigns')}
              {navLink('/create', 'Create')}
              {navLink('/profile', 'Profile')}
            </nav>
          </div>

          <div className="flex items-center gap-3">
            {user ? (
              <div className="flex items-center gap-3">
                <Link
                  to="/profile"
                  className="flex items-center gap-2 hover:opacity-80 transition-opacity duration-200"
                >
                  <img
                    src={user.avatar_url}
                    alt={user.github_username}
                    className="w-7 h-7 rounded-full ring-1 ring-solana-border"
                  />
                  <span className="text-xs text-gray-400 hidden sm:inline">
                    {user.github_username}
                  </span>
                </Link>
                <button
                  onClick={logout}
                  className="text-xs text-gray-600 hover:text-gray-300 transition-colors"
                >
                  Logout
                </button>
              </div>
            ) : (
              <GitHubLoginButton />
            )}
            <WalletButton />
          </div>
        </div>

        {/* Mobile nav */}
        <div className="md:hidden border-t border-solana-border px-6 py-2 flex items-center gap-1">
          {navLink('/', 'Campaigns')}
          {navLink('/create', 'Create')}
          {navLink('/profile', 'Profile')}
        </div>
      </header>

      <main className="flex-1 max-w-6xl mx-auto px-6 py-8 w-full">{children}</main>

      <footer className="border-t border-solana-border py-5">
        <div className="max-w-6xl mx-auto px-6 flex items-center justify-between">
          <span className="text-xs text-gray-600">
            &copy; {new Date().getFullYear()} Enshor
          </span>
          <div className="flex items-center gap-4 text-xs text-gray-600">
            <span>Built on Solana</span>
            <a
              href="https://github.com"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-gray-400 transition-colors"
            >
              GitHub
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
