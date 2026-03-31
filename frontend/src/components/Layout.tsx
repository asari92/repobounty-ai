import { Link, useLocation } from 'react-router-dom';
import WalletButton from './WalletButton';
import { GitHubLoginButton } from './GitHubLoginButton';
import { useAuth } from '../hooks/useAuth';

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { user, logout } = useAuth();

  return (
    <div className="min-h-screen flex flex-col">
      <header className="border-b border-solana-border">
        <div className="max-w-6xl mx-auto px-6 py-4 flex items-center justify-between">
          <Link to="/" className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-solana-purple to-solana-green flex items-center justify-center font-bold text-sm">
              RB
            </div>
            <span className="text-xl font-bold gradient-text">RepoBounty AI</span>
          </Link>

          <nav className="flex items-center gap-6">
            <Link
              to="/"
              className={`text-sm transition-colors ${
                location.pathname === '/' ? 'text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              Campaigns
            </Link>
            <Link
              to="/create"
              className={`text-sm transition-colors ${
                location.pathname === '/create' ? 'text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              Create
            </Link>
            <Link
              to="/profile"
              className={`text-sm transition-colors ${
                location.pathname === '/profile' ? 'text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              Profile
            </Link>
            <div className="flex items-center gap-3 border-l border-solana-border pl-6">
              {user ? (
                <div className="flex items-center gap-3">
                  <img
                    src={user.avatar_url}
                    alt={user.github_username}
                    className="w-8 h-8 rounded-full"
                  />
                  <span className="text-sm text-solana-green">@{user.github_username}</span>
                  <button
                    onClick={logout}
                    className="text-sm text-gray-400 hover:text-white transition-colors"
                  >
                    Logout
                  </button>
                </div>
              ) : (
                <GitHubLoginButton />
              )}
              <WalletButton />
            </div>
          </nav>
        </div>
      </header>

      <main className="flex-1 max-w-6xl mx-auto px-6 py-8 w-full">{children}</main>

      <footer className="border-t border-solana-border py-6">
        <div className="max-w-6xl mx-auto px-6 text-center text-sm text-gray-500">
          RepoBounty AI — National Solana Hackathon (Decentrathon)
        </div>
      </footer>
    </div>
  );
}
