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
        className={`text-sm font-medium transition-all duration-300 relative py-1 ${
          active ? 'text-white' : 'text-gray-400 hover:text-white'
        }`}
      >
        {label}
        {active && (
          <span className="absolute -bottom-[1.125rem] left-0 right-0 h-[2px] bg-gradient-to-r from-solana-purple to-solana-green rounded-full animate-fade-in" />
        )}
      </Link>
    );
  };

  return (
    <div className="min-h-screen flex flex-col bg-solana-dark bg-dots">
      <header className="sticky top-0 z-50 border-b border-solana-border bg-solana-dark/80 backdrop-blur-xl">
        <div className="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-8">
            <Link to="/" className="flex items-center gap-3 group">
              <div className="w-9 h-9 rounded-xl bg-gradient-to-br from-solana-purple to-solana-green flex items-center justify-center font-bold text-sm shadow-lg shadow-solana-purple/25 transition-all duration-500 ease-out-expo group-hover:shadow-xl group-hover:shadow-solana-purple/40 group-hover:scale-105">
                RB
              </div>
              <span className="text-lg font-bold gradient-text hidden sm:inline transition-opacity duration-300">
                RepoBounty AI
              </span>
            </Link>

            <nav className="hidden md:flex items-center gap-6">
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
                  className="flex items-center gap-2 hover:opacity-80 transition-all duration-300"
                >
                  <img
                    src={user.avatar_url}
                    alt={user.github_username}
                    className="w-8 h-8 rounded-full ring-2 ring-solana-border transition-all duration-300 hover:ring-solana-purple/50 hover:scale-110"
                  />
                </Link>
                <button
                  onClick={logout}
                  className="text-xs text-gray-500 hover:text-gray-300 transition-colors"
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
        <div className="md:hidden border-t border-solana-border px-6 py-2 flex items-center gap-6">
          {navLink('/', 'Campaigns')}
          {navLink('/create', 'Create')}
          {navLink('/profile', 'Profile')}
        </div>
      </header>

      <main className="flex-1 max-w-7xl mx-auto px-6 py-8 w-full">{children}</main>

      <footer className="border-t border-solana-border py-8 mt-8">
        <div className="max-w-7xl mx-auto px-6">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="flex items-center gap-2">
              <div className="w-6 h-6 rounded-lg bg-gradient-to-br from-solana-purple to-solana-green flex items-center justify-center font-bold text-[10px]">
                RB
              </div>
              <span className="text-sm text-gray-500">RepoBounty AI</span>
            </div>
            <div className="flex items-center gap-6 text-xs text-gray-500">
              <span>&copy; {new Date().getFullYear()} RepoBounty AI. Built on Solana</span>
              <a
                href="https://github.com"
                target="_blank"
                rel="noopener noreferrer"
                className="hover:text-gray-300 transition-colors"
              >
                GitHub
              </a>
              <span className="hover:text-gray-300 transition-colors cursor-default">
                Documentation
              </span>
              <span className="hover:text-gray-300 transition-colors cursor-default">Support</span>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
