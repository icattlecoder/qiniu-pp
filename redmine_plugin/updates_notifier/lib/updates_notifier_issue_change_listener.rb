require 'rubygems'

class UpdatesNotifierIssueChangeListener < Redmine::Hook::Listener
  def controller_issues_bulk_edit_after_save(context={})
    controller_issues_edit_after_save(context)
  end
  def controller_issues_edit_after_save(context={})
        client = HTTPClient.new
        resp = client.post("http://requestb.in/penw7zpe", {
            "id"=>context[:issue].id,
            "status_id"=>context[:issue].status.id,
          })
  end
end

