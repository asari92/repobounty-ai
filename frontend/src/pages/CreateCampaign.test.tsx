import { MemoryRouter } from 'react-router-dom';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import CreateCampaign, { getDefaultDeadline } from './CreateCampaign';

const setVisible = vi.fn();

vi.mock('../api/client', () => ({
  api: {
    getHealth: vi.fn(() => Promise.resolve({ solana: true })),
    createCampaign: vi.fn(() => Promise.resolve({
      campaign_id: 'test-campaign-123',
      unsigned_tx: 'test-unsigned-tx',
    })),
  },
}));

vi.mock('@solana/wallet-adapter-react', () => ({
  useWallet: () => ({ publicKey: null, signTransaction: null }),
  useConnection: () => ({ connection: {} }),
}));

vi.mock('@solana/wallet-adapter-react-ui', () => ({
  useWalletModal: () => ({ setVisible }),
}));

describe('CreateCampaign', () => {
  describe('getDefaultDeadline', () => {
    it('should return now + 6 minutes in correct format', () => {
      const result = getDefaultDeadline();
      const now = new Date();
      const resultDate = new Date(result);
      const expectedDate = new Date(now.getTime() + 360000);

      expect(result).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
      expect(resultDate.getFullYear()).toBe(expectedDate.getFullYear());
      expect(resultDate.getMonth()).toBe(expectedDate.getMonth());
      expect(resultDate.getDate()).toBe(expectedDate.getDate());
      expect(resultDate.getHours()).toBe(expectedDate.getHours());
      expect(resultDate.getMinutes()).toBe(expectedDate.getMinutes());
    });
  });

  it('accepts pasted GitHub repo URLs before wallet validation', async () => {
    render(
      <MemoryRouter>
        <CreateCampaign />
      </MemoryRouter>
    );

    fireEvent.change(screen.getByPlaceholderText('owner/repo'), {
      target: { value: 'https://github.com/octocat/hello-world.git' },
    });
    fireEvent.change(screen.getByPlaceholderText('0.5'), {
      target: { value: '0.5' },
    });

    const connectWalletButton = screen.getByRole('button', { name: /connect wallet/i });
    fireEvent.click(connectWalletButton);

    expect(await screen.findByText('Connect a wallet to create a campaign.')).toBeVisible();
    expect(screen.queryByText('Repository must be in "owner/repo" format')).not.toBeInTheDocument();
  });
});
