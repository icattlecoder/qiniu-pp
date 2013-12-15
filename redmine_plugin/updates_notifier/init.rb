
require 'redmine'

require_dependency 'updates_notifier_issue_change_listener'

Redmine::Plugin.register :redmine_updates_notifier do
  name 'Redmine Updates Notifier plugin'
  author 'Ramesh Nair'
  description 'This sends update notifications to a callback URL when changes are made within Redmine.'
  version '0.0.1'
  url 'http://example.com/path/to/plugin'
  author_url 'http://example.com/about'

  settings :default => {'callback_url' => 'http://requestb.in/penw7zpe' },
      :partial => 'settings/updates_notifier_settings'

end


