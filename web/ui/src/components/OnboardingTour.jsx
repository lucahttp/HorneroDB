// Tours disabled - they were causing issues
// TODO: Reimplement with a simpler approach if needed

export function OnboardingTour() {
  return null; // Disabled
}

export function TableEditTour() {
  return null; // Disabled  
}

export function DashboardTour() {
  return null; // Disabled
}

// Keep reset function for cleanup
export const resetTour = () => {
  localStorage.removeItem('hornero-tour-seen');
  localStorage.removeItem('hornero-tour-completed');
  localStorage.removeItem('hornero-table-edit-tour-seen');
  localStorage.removeItem('hornero-table-edit-tour-completed');
  localStorage.removeItem('hornero-dashboard-tour-seen');
  localStorage.removeItem('hornero-dashboard-visited');
  localStorage.removeItem('hornero-visited');
  console.log('All tour data cleared');
};

export default OnboardingTour;
