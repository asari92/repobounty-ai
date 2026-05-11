export default function About() {
  return (
    <div className="space-y-16">
      <section className="flex flex-col md:flex-row gap-8 md:gap-12 items-center animate-fade-in-up">
        <div className="flex-1">
          <h1 className="text-3xl md:text-4xl font-bold tracking-tight mb-3">
            <span className="gradient-text">About Enshor</span>
          </h1>
          <p className="text-lg text-gray-300 mb-4">
            AI-powered rewards for open-source contributors on Solana.
          </p>
          <p className="text-sm text-gray-400 leading-relaxed max-w-xl">
            Enshor helps sponsors create reward campaigns for public GitHub
            repositories, analyze contributor impact with AI, and distribute
            rewards through Solana-powered on-chain claims.
          </p>
        </div>
        <div className="flex-1 max-w-md w-full">
          <img
            src="/brand/enshor-about-visual.png"
            alt="Enshor visual"
            className="w-full rounded-xl"
          />
        </div>
      </section>

      <section
        className="animate-fade-in-up"
        style={{ animationDelay: '80ms' }}
      >
        <h2 className="text-xl font-semibold text-white mb-2">
          What Enshor does
        </h2>
        <div className="gradient-line mb-4 max-w-xs" />
        <div className="max-w-3xl space-y-3 text-sm text-gray-400 leading-relaxed">
          <p>
            Open-source projects create real value, but rewarding contributors
            is still mostly manual, inconsistent, or completely missing.
          </p>
          <p>
            A sponsor may want to support a GitHub repository, but it is hard to
            decide who contributed the most, how much each person should receive,
            and how to make the process transparent.
          </p>
          <p>
            Enshor turns this into a clear workflow: create a campaign, lock the
            reward, analyze contributions, and distribute payouts to contributors.
          </p>
        </div>
      </section>

      <section
        className="animate-fade-in-up"
        style={{ animationDelay: '160ms' }}
      >
        <h2 className="text-xl font-semibold text-white mb-2">
          Why the name Enshor
        </h2>
        <div className="gradient-line mb-4 max-w-xs" />
        <div className="max-w-3xl space-y-3 text-sm text-gray-400 leading-relaxed">
          <p>
            Enshor is a compact expression of a powerful idea: everyone who
            creates real value deserves their part.
          </p>
          <p>
            The name is inspired by &ldquo;енші&rdquo;, the idea of a share that
            belongs to someone by right. We shaped it into a shorter, cleaner
            form that feels sharp, modern, and product-ready.
          </p>
          <p>
            We chose Enshor because we wanted a name that feels simple on the
            surface, but carries a deeper philosophy underneath. Our platform
            exists to make contribution visible and reward it fairly, even when
            the contributor was not working for reward in the first place.
          </p>
          <p>Enshor is about rightful recognition made practical.</p>
        </div>
      </section>

      <section
        className="animate-fade-in-up"
        style={{ animationDelay: '240ms' }}
      >
        <h2 className="text-xl font-semibold text-white mb-2">How it works</h2>
        <div className="gradient-line mb-4 max-w-xs" />
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="card">
            <h3 className="text-sm font-semibold text-white mb-1.5">
              Create a campaign
            </h3>
            <p className="text-xs text-gray-400 leading-relaxed">
              A sponsor selects a public GitHub repository, sets a reward pool,
              and creates a campaign.
            </p>
          </div>
          <div className="card">
            <h3 className="text-sm font-semibold text-white mb-1.5">
              Lock the reward
            </h3>
            <p className="text-xs text-gray-400 leading-relaxed">
              The reward is locked in a Solana escrow, making the campaign
              transparent and verifiable.
            </p>
          </div>
          <div className="card">
            <h3 className="text-sm font-semibold text-white mb-1.5">
              Analyze contribution
            </h3>
            <p className="text-xs text-gray-400 leading-relaxed">
              Enshor analyzes GitHub activity and uses AI to estimate contributor
              impact.
            </p>
          </div>
          <div className="card">
            <h3 className="text-sm font-semibold text-white mb-1.5">
              Claim rewards
            </h3>
            <p className="text-xs text-gray-400 leading-relaxed">
              Final allocations are written on-chain, and contributors can claim
              their rewards with a Solana wallet.
            </p>
          </div>
        </div>
      </section>

      <section
        className="animate-fade-in-up"
        style={{ animationDelay: '320ms' }}
      >
        <h2 className="text-xl font-semibold text-white mb-2">Why Solana</h2>
        <div className="gradient-line mb-4 max-w-xs" />
        <div className="max-w-3xl space-y-3 text-sm text-gray-400 leading-relaxed">
          <p>
            Solana gives Enshor the infrastructure needed for fast, low-cost, and
            transparent reward distribution.
          </p>
          <p>
            The MVP uses Solana not as a decorative Web3 layer, but as the core
            settlement system: campaign funds are locked in escrow, final
            allocations affect smart contract state, and contributors claim
            rewards through on-chain transactions.
          </p>
        </div>
      </section>

      <section
        className="animate-fade-in-up"
        style={{ animationDelay: '400ms' }}
      >
        <h2 className="text-xl font-semibold text-white mb-2">MVP scope</h2>
        <div className="gradient-line mb-4 max-w-xs" />
        <div className="max-w-3xl space-y-3 text-sm text-gray-400 leading-relaxed">
          <p>
            The MVP demonstrates the full chain: GitHub data → AI decision →
            on-chain transaction → smart contract state change.
          </p>
          <p>
            This is the key idea behind Enshor: AI helps evaluate contribution,
            while Solana makes the reward process transparent, executable, and
            verifiable.
          </p>
        </div>
      </section>
    </div>
  );
}
