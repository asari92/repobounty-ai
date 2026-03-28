import { Routes, Route } from "react-router-dom";
import ErrorBoundary from "./components/ErrorBoundary";
import Layout from "./components/Layout";
import Home from "./pages/Home";
import CreateCampaign from "./pages/CreateCampaign";
import CampaignDetails from "./pages/CampaignDetails";

export default function App() {
  return (
    <ErrorBoundary
      onError={(error, errorInfo) => {
        console.error('Global error caught:', error, errorInfo);
        if (process.env.NODE_ENV === 'production') {
        }
      }}
    >
      <Layout>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/create" element={<CreateCampaign />} />
          <Route path="/campaign/:id" element={<CampaignDetails />} />
        </Routes>
      </Layout>
    </ErrorBoundary>
  );
}
